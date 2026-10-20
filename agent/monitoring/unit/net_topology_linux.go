//go:build linux

package monitoring

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

var (
	bootIDOnce sync.Once
	bootIDVal  string
)

// getSystemBootID 读取 Linux 系统引导唯一 ID（开机固定，重启会重新生成）
func getSystemBootID() string {
	bootIDOnce.Do(func() {
		if data, err := os.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
			bootIDVal = strings.TrimSpace(string(data))
		}
		if bootIDVal == "" {
			bootIDVal = "unknown-boot-id"
		}
	})
	return bootIDVal
}

const (
	topologyCacheTTL   = 5 * time.Second
	topologyCacheLimit = 256
)

type topologyCacheEntry struct {
	countedElsewhere bool
	at               time.Time
}

type topologyCache struct {
	mu        sync.Mutex
	hostProc  string
	sampleKey string
	entries   map[string]topologyCacheEntry
	now       func() time.Time
	inspect   func(string, string) (bool, bool)
}

var networkTopologyCache topologyCache

// refreshNetworkTopology invalidates using the already-read counter names, never a sysfs walk.
func refreshNetworkTopology(counters []net.IOCountersStat, include, exclude map[string]struct{}) {
	names := make([]string, 0, len(counters))
	for _, counter := range counters {
		names = append(names, counter.Name)
	}
	includeNames := make([]string, 0, len(include))
	for name := range include {
		includeNames = append(includeNames, name)
	}
	excludeNames := make([]string, 0, len(exclude))
	for name := range exclude {
		excludeNames = append(excludeNames, name)
	}
	key := calcNicEpoch("", names) + calcNicEpoch("", includeNames) + calcNicEpoch("", excludeNames)
	networkTopologyCache.observeSample(flags.HostProc, key)
}

func (c *topologyCache) observeSample(hostProc, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hostProc != hostProc || c.sampleKey != key {
		c.entries = nil
		c.hostProc = hostProc
		c.sampleKey = key
	}
}

// isCountedElsewhere caches classification only; explicit filters retain their priority.
func isCountedElsewhere(nicName string) bool {
	return networkTopologyCache.classify(nicName, flags.HostProc)
}

func (c *topologyCache) classify(nicName, hostProc string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || c.hostProc != hostProc {
		c.hostProc = hostProc
		c.entries = make(map[string]topologyCacheEntry)
	}
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	if entry, ok := c.entries[nicName]; ok && !now.Before(entry.at) && now.Sub(entry.at) < topologyCacheTTL {
		return entry.countedElsewhere
	}
	inspect := c.inspect
	if inspect == nil {
		inspect = inspectNetworkTopology
	}
	result, cacheable := inspect(nicName, hostProc)
	if !cacheable {
		delete(c.entries, nicName)
		return result
	}
	if len(c.entries) >= topologyCacheLimit {
		// Bounded eviction; do not scan sysfs or retain historical interfaces.
		for name := range c.entries {
			delete(c.entries, name)
			break
		}
	}
	c.entries[nicName] = topologyCacheEntry{countedElsewhere: result, at: now}
	return result
}

// inspectNetworkTopology follows the existing Linux sysfs classification order.
func inspectNetworkTopology(nicName, hostProc string) (bool, bool) {
	cacheable := true
	sysNetDir := "/sys/class/net"
	if hostProc != "" {
		// 若配置了自定义 proc 路径，检查对应的 sys
		altSys := filepath.Join(filepath.Dir(hostProc), "sys/class/net")
		if st, err := os.Stat(altSys); err == nil && st.IsDir() {
			sysNetDir = altSys
		} else if err != nil && !os.IsNotExist(err) {
			// Preserve host fallback, but retry transient mount/permission errors next sample.
			cacheable = false
		}
	}

	devPath := filepath.Join(sysNetDir, nicName)
	if _, err := os.Stat(devPath); err != nil {
		return false, false
	}

	// 1. 检查是否存在 lower_* 符号链接：
	// 如果一个接口有 lower_*，说明它由底层物理设备组合/叠加而成 (如 bond、bridge、vlan、macvlan)。
	// 这些底层设备本身会计入物理流量，因此上层叠加网卡必须被跳过，防止同一数据包双重计数。
	entries, err := os.ReadDir(devPath)
	if err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "lower_") {
				return true, cacheable
			}
		}
	} else {
		cacheable = false
	}

	// 检查 uevent 里的 DEVTYPE 是否属于典型叠加类型
	if ueventData, err := os.ReadFile(filepath.Join(devPath, "uevent")); err == nil {
		uevent := string(ueventData)
		if strings.Contains(uevent, "DEVTYPE=bridge") ||
			strings.Contains(uevent, "DEVTYPE=bond") ||
			strings.Contains(uevent, "DEVTYPE=vlan") ||
			strings.Contains(uevent, "DEVTYPE=macvlan") ||
			strings.Contains(uevent, "DEVTYPE=ipvlan") {
			return true, cacheable
		}
	} else {
		cacheable = false
	}

	// 2. 检查是否有硬件物理总线关联：
	// 如果存在 /device 指向真实总线 (PCI / USB / VirtIO 等硬件设备)，说明是底层真实网络设备，不应排除！
	if _, err := os.Stat(filepath.Join(devPath, "device")); err == nil {
		return false, cacheable
	} else if !os.IsNotExist(err) {
		cacheable = false
	}

	// 3. 无底层硬件设备时的虚拟端口过滤：
	// 如果存在 brport 目录或者 master 符号链接，说明它是挂载在网桥后面的端口 (如 Docker 容器在宿主机的 veth 端口，或 KVM 的 tap 端口)。
	// 其流量会在物理出入网卡处被自然统计，故作为网桥从属端口应被跳过。
	if _, err := os.Stat(filepath.Join(devPath, "brport")); err == nil {
		return true, cacheable
	} else if !os.IsNotExist(err) {
		cacheable = false
	}
	if _, err := os.Stat(filepath.Join(devPath, "master")); err == nil {
		return true, cacheable
	} else if !os.IsNotExist(err) {
		cacheable = false
	}

	// 4. 检查隧道类型 (type: ARPHRD_TUNNEL / GRE / SIT / IPIP / WireGuard 等)
	if typeData, err := os.ReadFile(filepath.Join(devPath, "type")); err == nil {
		t := strings.TrimSpace(string(typeData))
		// 768=IP-tunnel, 776=SIT, 778=GRE, 820=WireGuard
		if t == "768" || t == "776" || t == "778" || t == "820" {
			return true, cacheable
		}
	} else {
		cacheable = false
	}

	return false, cacheable
}
