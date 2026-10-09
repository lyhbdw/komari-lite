//go:build linux

package monitoring

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// isCountedElsewhere 利用 Linux 内核 sysfs 拓扑判断网卡是否属于上层叠加网卡或从属端口。
// 避免在宿主机包含容器 (veth)、虚拟机 (tap)、网桥 (bridge)、聚合 (bond) 时导致同一数据包被重复计数。
func isCountedElsewhere(nicName string) bool {
	sysNetDir := "/sys/class/net"
	if flags.HostProc != "" {
		// 若配置了自定义 proc 路径，检查对应的 sys
		altSys := filepath.Join(filepath.Dir(flags.HostProc), "sys/class/net")
		if st, err := os.Stat(altSys); err == nil && st.IsDir() {
			sysNetDir = altSys
		}
	}

	devPath := filepath.Join(sysNetDir, nicName)
	if _, err := os.Stat(devPath); err != nil {
		return false
	}

	// 1. 检查是否存在 lower_* 符号链接：
	// 如果一个接口有 lower_*，说明它由底层物理设备组合/叠加而成 (如 bond、bridge、vlan、macvlan)。
	// 这些底层设备本身会计入物理流量，因此上层叠加网卡必须被跳过，防止同一数据包双重计数。
	entries, err := os.ReadDir(devPath)
	if err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "lower_") {
				return true
			}
		}
	}

	// 检查 uevent 里的 DEVTYPE 是否属于典型叠加类型
	if ueventData, err := os.ReadFile(filepath.Join(devPath, "uevent")); err == nil {
		uevent := string(ueventData)
		if strings.Contains(uevent, "DEVTYPE=bridge") ||
			strings.Contains(uevent, "DEVTYPE=bond") ||
			strings.Contains(uevent, "DEVTYPE=vlan") ||
			strings.Contains(uevent, "DEVTYPE=macvlan") ||
			strings.Contains(uevent, "DEVTYPE=ipvlan") {
			return true
		}
	}

	// 2. 检查是否有硬件物理总线关联：
	// 如果存在 /device 指向真实总线 (PCI / USB / VirtIO 等硬件设备)，说明是底层真实网络设备，不应排除！
	if _, err := os.Stat(filepath.Join(devPath, "device")); err == nil {
		return false
	}

	// 3. 无底层硬件设备时的虚拟端口过滤：
	// 如果存在 brport 目录或者 master 符号链接，说明它是挂载在网桥后面的端口 (如 Docker 容器在宿主机的 veth 端口，或 KVM 的 tap 端口)。
	// 其流量会在物理出入网卡处被自然统计，故作为网桥从属端口应被跳过。
	if _, err := os.Stat(filepath.Join(devPath, "brport")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(devPath, "master")); err == nil {
		return true
	}

	// 4. 检查隧道类型 (type: ARPHRD_TUNNEL / GRE / SIT / IPIP / WireGuard 等)
	if typeData, err := os.ReadFile(filepath.Join(devPath, "type")); err == nil {
		t := strings.TrimSpace(string(typeData))
		// 768=IP-tunnel, 776=SIT, 778=GRE, 820=WireGuard
		if t == "768" || t == "776" || t == "778" || t == "820" {
			return true
		}
	}

	return false
}
