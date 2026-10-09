package monitoring

import (
	"fmt"
	"os"
)

func Uptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	var up float64
	if _, err := fmt.Sscanf(string(data), "%f", &up); err != nil {
		return 0, err
	}
	return uint64(up), nil
}
