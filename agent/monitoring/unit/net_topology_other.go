//go:build !linux

package monitoring

import "github.com/shirou/gopsutil/v4/net"

func refreshNetworkTopology(counters []net.IOCountersStat, include, exclude map[string]struct{}) {}

func getSystemBootID() string {
	return "generic-boot-id"
}

func isCountedElsewhere(nicName string) bool {
	return false
}
