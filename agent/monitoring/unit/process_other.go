//go:build !linux

package monitoring

import (
	"github.com/shirou/gopsutil/v4/process"
)

// ProcessCount returns the number of running processes on non-Linux platforms
func ProcessCount() (count int) {
	pids, err := process.Pids()
	if err != nil {
		return 0
	}
	return len(pids)
}
