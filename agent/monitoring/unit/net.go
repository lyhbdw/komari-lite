package monitoring

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/agent/monitoring/netstatic"
	"github.com/lyhbdw/komari-lite/agent/utils"
	"github.com/shirou/gopsutil/v4/net"
)

func ConnectionsCount() (tcpCount, udpCount int, err error) {
	if runtime.GOOS == "linux" {
		return connectionsCountWithProcFallback(procRoot(), gopsutilConnectionsCount)
	}

	return gopsutilConnectionsCount()
}

func connectionsCountWithProcFallback(root string, fallback func() (int, int, error)) (tcpCount, udpCount int, err error) {
	var procErr error
	tcpCount, udpCount, procErr = procNetConnectionsCount(root)
	if procErr == nil {
		return tcpCount, udpCount, nil
	}

	tcpCount, udpCount, err = fallback()
	if err != nil && procErr != nil {
		return 0, 0, fmt.Errorf("proc net fast path failed: %w; gopsutil fallback failed: %w", procErr, err)
	}
	return tcpCount, udpCount, err
}

func gopsutilConnectionsCount() (tcpCount, udpCount int, err error) {
	tcps, err := net.Connections("tcp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get TCP connections: %w", err)
	}
	udps, err := net.Connections("udp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get UDP connections: %w", err)
	}

	return len(tcps), len(udps), nil
}

func procRoot() string {
	if flags.HostProc != "" {
		return flags.HostProc
	}
	return "/proc"
}

func procNetConnectionsCount(root string) (tcpCount, udpCount int, err error) {
	// 极速快路径：优先尝试解析 /proc/net/sockstat 和 sockstat6
	// 内核直接提供了聚合统计数据（仅数行文本），无需逐行扫描数万条连接大表，CPU 与内存开销下降 90% 以上
	if tcp, udp, ok := procNetSockstatCount(root); ok {
		return tcp, udp, nil
	}

	tcpCount, err = countProcNetFiles(root, "tcp", "tcp6")
	if err != nil {
		return 0, 0, err
	}
	udpCount, err = countProcNetFiles(root, "udp", "udp6")
	if err != nil {
		return 0, 0, err
	}
	return tcpCount, udpCount, nil
}

// procNetSockstatCount 从 sockstat / sockstat6 解析当前活跃套接字汇总
func procNetSockstatCount(root string) (tcpCount, udpCount int, ok bool) {
	parsedAny := false

	// IPv4: /proc/net/sockstat
	if data, err := os.ReadFile(filepath.Join(root, "net", "sockstat")); err == nil {
		parsedAny = true
		t, u := parseSockstatLines(data)
		tcpCount += t
		udpCount += u
	}

	// IPv6: /proc/net/sockstat6 (部分环境可能无 IPv6，允许不存在)
	if data, err := os.ReadFile(filepath.Join(root, "net", "sockstat6")); err == nil {
		parsedAny = true
		t, u := parseSockstatLines(data)
		tcpCount += t
		udpCount += u
	}

	return tcpCount, udpCount, parsedAny
}

func parseSockstatLines(data []byte) (tcp, udp int) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Bytes()
		// 格式形如: "TCP: inuse 118 orphan 0 tw 16 alloc 225 mem 0" 或 "TCP6: inuse 67"
		if bytes.HasPrefix(line, []byte("TCP:")) || bytes.HasPrefix(line, []byte("TCP6:")) {
			fields := bytes.Fields(line)
			for i := 0; i < len(fields)-1; i++ {
				if bytes.Equal(fields[i], []byte("inuse")) {
					var val int
					if _, err := fmt.Sscanf(string(fields[i+1]), "%d", &val); err == nil {
						tcp = val
					}
					break
				}
			}
		} else if bytes.HasPrefix(line, []byte("UDP:")) || bytes.HasPrefix(line, []byte("UDP6:")) {
			// 格式形如: "UDP: inuse 0 mem 2" 或 "UDP6: inuse 1"
			fields := bytes.Fields(line)
			for i := 0; i < len(fields)-1; i++ {
				if bytes.Equal(fields[i], []byte("inuse")) {
					var val int
					if _, err := fmt.Sscanf(string(fields[i+1]), "%d", &val); err == nil {
						udp = val
					}
					break
				}
			}
		}
	}
	return tcp, udp
}

func countProcNetFiles(root string, names ...string) (int, error) {
	total := 0
	readAny := false
	for _, name := range names {
		count, err := countProcNetFile(filepath.Join(root, "net", name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		total += count
		readAny = true
	}
	if !readAny {
		return 0, fmt.Errorf("no proc net files found under %s", filepath.Join(root, "net"))
	}
	return total, nil
}

func countProcNetFile(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	header := true
	for scanner.Scan() {
		if header {
			header = false
			continue
		}
		if len(bytes.TrimSpace(scanner.Bytes())) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}

var (
	// 预定义常见的回环和虚拟接口名称
	loopbackNames = map[string]struct{}{
		"br":      {},
		"cni":     {},
		"docker":  {},
		"podman":  {},
		"flannel": {},
		"lo":      {},
		"veth":    {}, // Docker
		"virbr":   {}, // KVM
		"vmbr":    {}, // Proxmox
		"tap":     {},
		"fwbr":    {},
		"fwpr":    {},
	}
)

func NetworkSpeed() (totalUp, totalDown, upSpeed, downSpeed uint64, err error) {
	includeNics := parseNics(flags.IncludeNics)
	excludeNics := parseNics(flags.ExcludeNics)

	// 如果设置了月重置（非0），统计totalUp、totalDown
	if flags.MonthRotate != 0 {
		netstatic.StartOrContinue() // 确保netstatic在运行
		now := uint64(time.Now().Unix())
		resetDay := uint64(utils.GetLastResetDate(flags.MonthRotate, time.Now()).Unix())
		nicStatics, err := netstatic.GetTotalTrafficBetween(resetDay, now)
		if err != nil {
			// 如果netstatic失败，回退到原来的方法，并返回额外的错误信息
			fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fallbackErr := getNetworkSpeedFallback(includeNics, excludeNics)
			if fallbackErr != nil {
				return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("failed to call GetTotalTrafficBetween: %v; fallback error: %w", err, fallbackErr)
			}
			return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("failed to call GetTotalTrafficBetween: %w", err)
		}

		for interfaceName, stats := range nicStatics {
			if shouldInclude(interfaceName, includeNics, excludeNics) {
				totalUp += stats.Tx
				totalDown += stats.Rx
			}
		}

		// 对于实时速度，仍然使用网卡累计计数器差值
		_, _, upSpeed, downSpeed, err = getNetworkSpeedFallback(includeNics, excludeNics)
		if err != nil {
			return totalUp, totalDown, 0, 0, err
		}

		return totalUp, totalDown, upSpeed, downSpeed, nil
	}

	// 如果没有设置月重置，使用原来的方法
	return getNetworkSpeedFallback(includeNics, excludeNics)
}

func getNetworkSpeedFallback(includeNics, excludeNics map[string]struct{}) (totalUp, totalDown, upSpeed, downSpeed uint64, err error) {
	totalUp, totalDown, countedNics, err := collectNetworkTotals(includeNics, excludeNics)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	epoch := calcNicEpoch(getSystemBootID(), countedNics)
	upSpeed, downSpeed = updateNetworkSpeedSample(totalUp, totalDown, epoch, time.Now())
	return totalUp, totalDown, upSpeed, downSpeed, nil
}

func collectNetworkTotals(includeNics, excludeNics map[string]struct{}) (totalUp, totalDown uint64, countedNics []string, err error) {
	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters) == 0 {
		return 0, 0, nil, fmt.Errorf("no network interfaces found")
	}

	for _, interfaceStats := range ioCounters {
		if shouldInclude(interfaceStats.Name, includeNics, excludeNics) {
			totalUp += interfaceStats.BytesSent
			totalDown += interfaceStats.BytesRecv
			countedNics = append(countedNics, interfaceStats.Name)
		}
	}

	return totalUp, totalDown, countedNics, nil
}

type networkSpeedState struct {
	sync.Mutex
	epoch     string
	totalUp   uint64
	totalDown uint64
	sampledAt time.Time
}

var networkSpeedSample networkSpeedState

// calcNicEpoch 组合 Linux 系统 boot_id 和当前统计网卡集合摘要，
// 当机器发生重启或网卡变动时触发基线重新对齐，杜绝历史计数器突变导致的流量暴增
func calcNicEpoch(bootID string, nics []string) string {
	sortedNics := make([]string, len(nics))
	copy(sortedNics, nics)
	sort.Strings(sortedNics)
	h := fnv.New64a()
	for _, n := range sortedNics {
		_, _ = h.Write([]byte(n))
		_, _ = h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("%s/%016x", bootID, h.Sum64())
}

func updateNetworkSpeedSample(totalUp, totalDown uint64, epoch string, now time.Time) (upSpeed, downSpeed uint64) {
	networkSpeedSample.Lock()
	defer networkSpeedSample.Unlock()

	// 首次采样，或检测到系统重启/网卡集合变动 (epoch 改变)：重置基准线 (Re-baseline)，不将既往累计误当作本周期流量
	if networkSpeedSample.sampledAt.IsZero() || networkSpeedSample.epoch != epoch {
		networkSpeedSample.epoch = epoch
		networkSpeedSample.totalUp = totalUp
		networkSpeedSample.totalDown = totalDown
		networkSpeedSample.sampledAt = now
		return 0, 0
	}

	elapsed := now.Sub(networkSpeedSample.sampledAt).Seconds()
	if elapsed <= 0 {
		return 0, 0
	}

	upDelta := safeCounterDelta(totalUp, networkSpeedSample.totalUp)
	downDelta := safeCounterDelta(totalDown, networkSpeedSample.totalDown)

	networkSpeedSample.totalUp = totalUp
	networkSpeedSample.totalDown = totalDown
	networkSpeedSample.sampledAt = now

	return uint64(float64(upDelta) / elapsed), uint64(float64(downDelta) / elapsed)
}

func safeCounterDelta(current, previous uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return 0
}

var (
	nicsCacheMu sync.RWMutex
	nicsCache   = make(map[string]map[string]struct{})
)

func parseNics(nics string) map[string]struct{} {
	if nics == "" {
		return nil
	}
	nicsCacheMu.RLock()
	if set, ok := nicsCache[nics]; ok {
		nicsCacheMu.RUnlock()
		return set
	}
	nicsCacheMu.RUnlock()

	nicSet := make(map[string]struct{})
	for _, nic := range strings.Split(nics, ",") {
		nicSet[strings.TrimSpace(nic)] = struct{}{}
	}

	nicsCacheMu.Lock()
	nicsCache[nics] = nicSet
	nicsCacheMu.Unlock()
	return nicSet
}

func shouldInclude(nicName string, includeNics, excludeNics map[string]struct{}) bool {
	// 无论如何，回环接口与常见虚拟回环必须排除（即使出现在 includeNics 中）
	if nicName == "lo" || strings.HasPrefix(nicName, "lo:") {
		return false
	}
	for loopbackName := range loopbackNames {
		if strings.HasPrefix(nicName, loopbackName) {
			return false
		}
	}

	// 如果定义了白名单，包含白名单中的接口
	for pattern := range includeNics {
		if matched, _ := filepath.Match(pattern, nicName); matched {
			return true
		}
	}

	// 如果定义了黑名单，排除黑名单中的接口
	for pattern := range excludeNics {
		if matched, _ := filepath.Match(pattern, nicName); matched {
			return false
		}
	}

	// 如果定义了白名单但未能匹配，则不包含
	if len(includeNics) > 0 {
		return false
	}

	// Linux 内核级拓扑判定：自动排除上层叠加网卡 (拥有 lower_* 链路的 bond/bridge/vlan/macvlan)
	// 以及挂载在网桥后的虚拟从属端口 (brport/master)，避免 Docker、虚拟机、网桥环境下流量重复统计
	if isCountedElsewhere(nicName) {
		return false
	}

	return true
}

func InterfaceList() ([]string, error) {
	includeNics := parseNics(flags.IncludeNics)
	excludeNics := parseNics(flags.ExcludeNics)
	interfaces := []string{}

	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}
	for _, interfaceStats := range ioCounters {
		if shouldInclude(interfaceStats.Name, includeNics, excludeNics) {
			interfaces = append(interfaces, interfaceStats.Name)
		}
	}
	return interfaces, nil
}
