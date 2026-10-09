//go:build !linux

package monitoring

import (
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/host"
)

func OSName() string {
	info, err := host.Info()
	if err == nil {
		if info.Platform != "" {
			if info.PlatformVersion != "" {
				return info.Platform + " " + info.PlatformVersion
			}
			return info.Platform
		}
		if info.OS != "" {
			return info.OS
		}
	}
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "freebsd":
		return "FreeBSD"
	case "openbsd":
		return "OpenBSD"
	default:
		if len(runtime.GOOS) > 0 {
			return strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
		}
		return "Unknown"
	}
}

func KernelVersion() string {
	info, err := host.Info()
	if err == nil && info.KernelVersion != "" {
		return info.KernelVersion
	}
	return "Unknown"
}
