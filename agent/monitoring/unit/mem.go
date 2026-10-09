package monitoring

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	pkg_flags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	"github.com/shirou/gopsutil/v4/mem"
)

var (
	procMemMu       sync.RWMutex
	procMemCached   *ProcMemInfo
	procMemCachedAt time.Time
)

type RamInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Mode  string
}

type ProcMemInfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
	Buffers      uint64
	Cached       uint64
	SwapTotal    uint64
	SwapFree     uint64
	SwapCached   uint64
	Shmem        uint64
	SReclaimable uint64
	Zswap        uint64
	Zswapped     uint64
}

// readProcMeminfo reads /proc/meminfo and returns a filled ProcMemInfo struct
func ReadProcMeminfo() (*ProcMemInfo, error) {
	procMemMu.RLock()
	if procMemCached != nil && time.Since(procMemCachedAt) < 500*time.Millisecond {
		info := *procMemCached
		procMemMu.RUnlock()
		return &info, nil
	}
	procMemMu.RUnlock()

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &ProcMemInfo{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx == -1 {
			continue
		}
		key := line[:colonIdx]
		valStr := strings.TrimSpace(line[colonIdx+1:])
		if spIdx := strings.IndexByte(valStr, ' '); spIdx != -1 {
			valStr = valStr[:spIdx]
		}
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		val *= 1024 // Convert kB to bytes

		switch key {
		case "MemTotal":
			info.MemTotal = val
		case "MemFree":
			info.MemFree = val
		case "MemAvailable":
			info.MemAvailable = val
		case "Buffers":
			info.Buffers = val
		case "Cached":
			info.Cached = val
		case "SwapTotal":
			info.SwapTotal = val
		case "SwapFree":
			info.SwapFree = val
		case "SwapCached":
			info.SwapCached = val
		case "Shmem":
			info.Shmem = val
		case "SReclaimable":
			info.SReclaimable = val
		case "Zswap":
			info.Zswap = val
		case "Zswapped":
			info.Zswapped = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	procMemMu.Lock()
	copied := *info
	procMemCached = &copied
	procMemCachedAt = time.Now()
	procMemMu.Unlock()

	return info, nil
}

func GetMemHtopLike() RamInfo {
	raminfo := RamInfo{Mode: "htoplike"}
	if runtime.GOOS == "linux" {
		info, err := ReadProcMeminfo()
		if err == nil && info.MemTotal > 0 {
			raminfo.Total = info.MemTotal
			// htop logic:
			// usedDiff = free + cached + sreclaimable + buffers
			usedDiff := info.MemFree + info.Cached + info.SReclaimable + info.Buffers

			if info.MemTotal >= usedDiff {
				raminfo.Used = info.MemTotal - usedDiff
			} else {
				raminfo.Used = info.MemTotal - info.MemFree
			}
			raminfo.Used += info.Shmem

			//if info.Zswap > 0 || info.Zswapped > 0 {
			//	if raminfo.Used > info.Zswap {
			//		raminfo.Used -= info.Zswap
			//	} else {
			//		raminfo.Used = 0
			//	}
			//}
			return raminfo
		}
	}
	return raminfo
}

func GetMemGopsutil() RamInfo {
	raminfo := RamInfo{Mode: "gopsutil"}
	v, err := mem.VirtualMemory()
	if err == nil {
		raminfo.Total = v.Total
		raminfo.Used = v.Total - v.Available
	}
	return raminfo
}

func Ram() RamInfo {
	// Use global config
	if pkg_flags.GlobalConfig.MemoryIncludeCache {
		v, err := mem.VirtualMemory()
		if err != nil {
			return RamInfo{}
		}
		return RamInfo{
			Total: v.Total,
			Used:  v.Total - v.Free,
			Mode:  "includeCache",
		}
	}

	if pkg_flags.GlobalConfig.MemoryReportRawUsed {
		return GetMemHtopLike()
	}

	if runtime.GOOS == "linux" {
		h := GetMemHtopLike()
		if h.Total > 0 {
			return h
		}
	}

	// Default fallback
	return GetMemGopsutil()
}

func Swap() RamInfo {
	swapinfo := RamInfo{}

	if runtime.GOOS == "linux" {
		info, err := ReadProcMeminfo()
		if err == nil {
			swapinfo.Total = info.SwapTotal
			// used = total - free - cached
			// Check for underflow
			usedDeductions := info.SwapFree + info.SwapCached
			if info.SwapTotal >= usedDeductions {
				swapinfo.Used = info.SwapTotal - usedDeductions
			} else {
				swapinfo.Used = info.SwapTotal - info.SwapFree
			}
			return swapinfo
		}
	}

	s, err := mem.SwapMemory()
	if err != nil {
		return swapinfo
	}
	swapinfo.Total = s.Total
	swapinfo.Used = s.Used
	return swapinfo
}
