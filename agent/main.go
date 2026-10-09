package main

import (
	"os"
	"runtime/debug"
	"time"

	"github.com/lyhbdw/komari-lite/agent/cmd"
)

func main() {
	// 针对当前守护进程进行平台特有的轻量化内存配置（如 Linux 下禁用 THP 大页虚高）
	initPlatformMemory()

	// 轻量化内存调优：探针作为轻量常驻后台进程，设置敏捷 GC 并约束内存软上限
	debug.SetGCPercent(25)
	debug.SetMemoryLimit(12 * 1024 * 1024)

	// 定期主动归还空闲页给操作系统，维持极低物理常驻内存 (RSS)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for range ticker.C {
			debug.FreeOSMemory()
		}
	}()

	cmd.Execute()
	os.Exit(0)
}
