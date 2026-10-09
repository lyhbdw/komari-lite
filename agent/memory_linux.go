//go:build linux

package main

import "golang.org/x/sys/unix"

func initPlatformMemory() {
	// 针对当前守护进程禁用透明大页 (THP)，防止内核强行按 2MB 大页合并导致 RSS 虚高 10MB+
	_ = unix.Prctl(unix.PR_SET_THP_DISABLE, 1, 0, 0, 0)
}
