package monitoring

import (
	"os"
	"strconv"
	"strings"
)

type LoadInfo struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

func Load() LoadInfo {
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			l1, e1 := strconv.ParseFloat(fields[0], 64)
			l5, e5 := strconv.ParseFloat(fields[1], 64)
			l15, e15 := strconv.ParseFloat(fields[2], 64)
			if e1 == nil && e5 == nil && e15 == nil {
				return LoadInfo{Load1: l1, Load5: l5, Load15: l15}
			}
		}
	}
	return LoadInfo{Load1: 0, Load5: 0, Load15: 0}
}
