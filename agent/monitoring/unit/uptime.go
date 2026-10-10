package monitoring

import (
	"os"
	"strconv"
	"strings"
)

func Uptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, os.ErrInvalid
	}
	up, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return uint64(up), nil
}
