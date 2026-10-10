package netstatic

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/agent/internal/netsample"
)

/*
统计每个网卡的流量情况，保存最近DataPreserveDay天的数据，每DetectInterval秒采集一次

默认保存到当前目录下的net_static.json文件中
net_static.json 中有字段 config，表示当前的配置，如果没有则使用默认值
unix时间戳，单位秒

所有操作都尽可能在内存中完成，避免频繁的IO操作

只有在启动、停止和保存时，才会进行文件的读写操作
*/
var (
	DefaultDataPreserveDay = 31.0      // in days，保存最近多少天的数据，过期数据会被删除
	DefaultDetectInterval  = 2.0       // in seconds，采集间隔
	DefaultSaveInterval    = 60.0 * 10 // in seconds，写入到磁盘的间隔，避免大量IO操作，保存到文件的间隔也是这个值，而不是DetectInterval
	SaveFilePath           = "./net_static.json"
)

var (
	staticCache map[string][]TrafficData // key: interface name，统计缓存，当前没有被保存到文件中的，间隔DetectInterval，触发保存时，合并所有的tx/rx数据，以SaveInterval，写入到文件中，随后清空缓存
	config      NetStaticConfig
)

// NetStatic 网卡流量统计数据
type NetStatic struct {
	Interfaces map[string][]TrafficData `json:"interfaces"` // key: interface name
	Config     NetStaticConfig          `json:"config"`
}

type NetStaticConfig struct {
	DataPreserveDay float64  `json:"data_preserve_day"` // in days，保存最近多少天的数据，过期数据会被删除
	DetectInterval  float64  `json:"detect_interval"`   // in seconds，采集间隔
	SaveInterval    float64  `json:"save_interval"`     // in seconds，写入到磁盘的间隔，避免大量IO操作
	Nics            []string `json:"nics"`              // 仅监控指定的网卡名称列表，空表示监控所有网卡
}

type TrafficData struct {
	Timestamp uint64 `json:"timestamp"`
	Tx        uint64 `json:"tx"` // 第n与n-1次采集的差值
	Rx        uint64 `json:"rx"` // 第n与n-1次采集的差值
}

var (
	mu          sync.RWMutex
	lifecycleMu sync.Mutex // Serializes transitions, never used by collectors.
	running     bool
	generation  *collectorGeneration

	// 内存持久区（与文件内容一致，但仅在启动、保存、停止时与磁盘交互）
	store NetStatic

	// 上次采集到的累计字节数（用于计算 delta）
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}
)

func nowUnix() uint64 { return uint64(time.Now().Unix()) }

// isNicAllowed 判断网卡是否在监控白名单内；当未配置白名单（空切片或nil）时，允许所有网卡
func isNicAllowed(name string) bool {
	if len(config.Nics) == 0 {
		return true
	}
	for _, n := range config.Nics {
		if n == name {
			return true
		}
	}
	return false
}

func ensureInitLocked() {
	if store.Interfaces == nil {
		store.Interfaces = make(map[string][]TrafficData)
	}
	if staticCache == nil {
		staticCache = make(map[string][]TrafficData)
	}
	if config.DataPreserveDay == 0 {
		config.DataPreserveDay = DefaultDataPreserveDay
	}
	if config.DetectInterval == 0 {
		config.DetectInterval = DefaultDetectInterval
	}
	if config.SaveInterval == 0 {
		config.SaveInterval = DefaultSaveInterval
	}
}

func loadFromFileLocked() error {
	invalidateRangeLocked()
	// 不存在则用默认配置
	f, err := os.Open(SaveFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ensureInitLocked()
			store.Config = configOrDefault(config)
			return nil
		}
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		ensureInitLocked()
		store.Config = configOrDefault(config)
		return nil
	}
	var ns NetStatic
	if err := json.Unmarshal(data, &ns); err != nil {
		// 文件损坏则不阻塞使用，采用默认并备份坏文件
		_ = os.Rename(SaveFilePath, SaveFilePath+".bak")
		ensureInitLocked()
		store.Config = configOrDefault(config)
		return nil
	}
	store = ns
	config = configOrDefault(ns.Config)
	ensureInitLocked()
	// 启动时清理过期数据
	purgeExpiredLocked()
	return nil
}

func saveToFileLocked() error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(SaveFilePath), 0o755); err != nil {
		return err
	}
	// 写入时带上当前 config
	store.Config = configOrDefault(config)
	b, err := json.Marshal(store) // 紧凑格式（不缩进）
	if err != nil {
		return err
	}
	tmp := SaveFilePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, SaveFilePath)
}

func configOrDefault(c NetStaticConfig) NetStaticConfig {
	if c.DataPreserveDay == 0 {
		c.DataPreserveDay = DefaultDataPreserveDay
	}
	if c.DetectInterval == 0 {
		c.DetectInterval = DefaultDetectInterval
	}
	if c.SaveInterval == 0 {
		c.SaveInterval = DefaultSaveInterval
	}
	return c
}

func purgeExpiredLocked() {
	invalidateRangeLocked()
	// 根据 DataPreserveDay 删除过期数据
	ttl := time.Duration(config.DataPreserveDay * 24 * float64(time.Hour))
	cutoff := uint64(time.Now().Add(-ttl).Unix())
	for name, arr := range store.Interfaces {
		// 仅保留 >= cutoff 的数据
		kept := arr[:0]
		for _, td := range arr {
			if td.Timestamp >= cutoff {
				kept = append(kept, td)
			}
		}
		if len(kept) == 0 {
			delete(store.Interfaces, name)
		} else {
			store.Interfaces[name] = kept
		}
	}
}

func safeDelta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	// 处理计数器回绕或重置，视为 0 增量
	return 0
}

func sampleOnceLocked() {
	ios, err := netsample.Counters()
	if err != nil {
		return
	}
	ts := nowUnix()
	for _, io := range ios {
		name := io.Name
		// 仅监控指定网卡（当配置了 Nics 时）
		if !isNicAllowed(name) {
			continue
		}
		curTx := io.BytesSent
		curRx := io.BytesRecv
		prev, ok := lastCounters[name]
		if ok {
			dtx := safeDelta(curTx, prev.Tx)
			drx := safeDelta(curRx, prev.Rx)
			// 首次采样不记录
			if dtx > 0 || drx > 0 {
				appendTrafficLocked(name, TrafficData{Timestamp: ts, Tx: dtx, Rx: drx})
			} else {
				// 即便为 0，也可以记录，但为了降低噪音与占用，这里忽略 0
			}
		}
		lastCounters[name] = struct{ Tx, Rx uint64 }{Tx: curTx, Rx: curRx}
	}
}

func flushCacheLocked(ts uint64) {
	// Persisted bins retain their existing save-time timestamps.
	invalidateRangeLocked()
	if len(staticCache) == 0 {
		return
	}
	for name, arr := range staticCache {
		var sumTx, sumRx uint64
		for _, td := range arr {
			sumTx += td.Tx
			sumRx += td.Rx
		}
		if sumTx > 0 || sumRx > 0 {
			store.Interfaces[name] = append(store.Interfaces[name], TrafficData{Timestamp: ts, Tx: sumTx, Rx: sumRx})
		}
	}
	// 清空缓存
	staticCache = make(map[string][]TrafficData)
}

type collectorGeneration struct {
	detect, save *time.Ticker
	stop         chan struct{}
	workers      sync.WaitGroup
}

// Each worker captures its own generation. Transitions wait outside mu.
func startGoroutinesLocked() {
	g := &collectorGeneration{
		detect: time.NewTicker(time.Duration(config.DetectInterval * float64(time.Second))),
		save:   time.NewTicker(time.Duration(config.SaveInterval * float64(time.Second))),
		stop:   make(chan struct{}),
	}
	generation = g
	g.workers.Add(2)
	go func() {
		defer g.workers.Done()
		for {
			select {
			case <-g.stop:
				return
			case <-g.detect.C:
				mu.Lock()
				select {
				case <-g.stop:
					mu.Unlock()
					return
				default:
				}
				sampleOnceLocked()
				mu.Unlock()
			}
		}
	}()
	go func() {
		defer g.workers.Done()
		for {
			select {
			case <-g.stop:
				return
			case t := <-g.save.C:
				mu.Lock()
				select {
				case <-g.stop:
					mu.Unlock()
					return
				default:
				}
				flushCacheLocked(uint64(t.Unix()))
				purgeExpiredLocked()
				_ = saveToFileLocked()
				mu.Unlock()
			}
		}
	}()
}

func stopGenerationLocked() *collectorGeneration {
	g := generation
	if g != nil {
		g.detect.Stop()
		g.save.Stop()
		close(g.stop)
		generation = nil
	}
	return g
}

func StartOrContinue() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	if running {
		return nil
	}
	ensureInitLocked()
	if err := loadFromFileLocked(); err != nil {
		return err
	}
	lastCounters = map[string]struct{ Tx, Rx uint64 }{} // Do not count stopped intervals as live samples.
	running = true
	startGoroutinesLocked()
	return nil
}

func Stop() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	mu.Lock()
	if !running {
		mu.Unlock()
		return nil
	}
	running = false
	g := stopGenerationLocked()
	mu.Unlock()
	if g != nil {
		g.workers.Wait()
	}
	mu.Lock()
	defer mu.Unlock()
	flushCacheLocked(nowUnix())
	purgeExpiredLocked()
	return saveToFileLocked()
}

// GetTotalTrafficBetween preserves inclusive endpoints and returns a defensive copy.
// An advancing end with the same start only processes new or future samples.
func GetTotalTrafficBetween(start, end uint64) (map[string]TrafficData, error) {
	mu.Lock()
	defer mu.Unlock()
	ensureInitLocked()
	return trafficRangeLocked(start, end), nil
}

// SetNewConfig keeps zero=unchanged and nil Nics=unchanged semantics.
func SetNewConfig(newCfg NetStaticConfig) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	mu.Lock()
	ensureInitLocked()
	wasRunning := running
	g := stopGenerationLocked()
	mu.Unlock()
	if g != nil {
		g.workers.Wait()
	}
	mu.Lock()
	defer mu.Unlock()
	if newCfg.DataPreserveDay != 0 {
		config.DataPreserveDay = newCfg.DataPreserveDay
	}
	if newCfg.DetectInterval != 0 {
		config.DetectInterval = newCfg.DetectInterval
	}
	if newCfg.SaveInterval != 0 {
		config.SaveInterval = newCfg.SaveInterval
	}
	if newCfg.Nics != nil {
		config.Nics = append([]string(nil), newCfg.Nics...)
		if len(config.Nics) > 0 {
			for name := range lastCounters {
				if !isNicAllowed(name) {
					delete(lastCounters, name)
				}
			}
			for name := range staticCache {
				if !isNicAllowed(name) {
					delete(staticCache, name)
				}
			}
		}
	}
	config = configOrDefault(config)
	store.Config = config
	invalidateRangeLocked()
	purgeExpiredLocked()
	if wasRunning {
		startGoroutinesLocked()
	}
	return saveToFileLocked()
}
