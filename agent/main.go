package main

import (
	"os"
	"runtime/debug"

	"github.com/lyhbdw/komari-lite/agent/cmd"
)

func main() {
	// 针对当前守护进程进行平台特有的轻量化内存配置（如 Linux 下禁用 THP 大页虚高）
	initPlatformMemory()

	configureRuntimeMemory()

	cmd.Execute()
	os.Exit(0)
}

// Keep the existing fallback policy without increasing GC aggressiveness.
// Explicit Go runtime environment settings are interpreted by Go at startup.
// Periodic FreeOSMemory is intentionally disabled: no measured RSS/CPU benefit.
func configureRuntimeMemory() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(25)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(12 << 20)
	}
}
