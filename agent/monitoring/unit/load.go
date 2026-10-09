package monitoring

import (
	"fmt"
	"os"
)

type LoadInfo struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

func Load() LoadInfo {
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		var l1, l5, l15 float64
		if _, err := fmt.Sscanf(string(data), "%f %f %f", &l1, &l5, &l15); err == nil {
			return LoadInfo{Load1: l1, Load5: l5, Load15: l15}
		}
	}
	return LoadInfo{Load1: 0, Load5: 0, Load15: 0}
}
