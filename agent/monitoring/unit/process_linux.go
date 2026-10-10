//go:build linux
// +build linux

package monitoring

import (
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	procCountMu       sync.RWMutex
	cachedProcCount   int
	procCountCachedAt time.Time
)

// ProcessCount returns the number of running processes
func ProcessCount() (count int) {
	procCountMu.RLock()
	if cachedProcCount > 0 && time.Since(procCountCachedAt) < 9*time.Second {
		c := cachedProcCount
		procCountMu.RUnlock()
		return c
	}
	procCountMu.RUnlock()

	c := processCountLinux()

	procCountMu.Lock()
	cachedProcCount = c
	procCountCachedAt = time.Now()
	procCountMu.Unlock()
	return c
}

// processCountLinux counts processes by reading /proc directory
func processCountLinux() (count int) {
	procDir := "/proc"

	if flags.HostProc != "" {
		if info, err := os.Stat(flags.HostProc); err == nil && info.IsDir() {
			procDir = flags.HostProc
		}
	}

	entries, err := os.ReadDir(procDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if _, err := strconv.ParseInt(entry.Name(), 10, 64); err == nil {
			count++
		}
	}

	return count
}
