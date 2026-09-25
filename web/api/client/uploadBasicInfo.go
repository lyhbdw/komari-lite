package client

import (
	"fmt"
	"github.com/Tumb1er1376/komari-monitor-lite/database/clients"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/config"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/geoip"
	"net"
	"strings"
)

func getClientIPType(ip net.IP) int {
	// 0:ipv4 1:ipv6 -1:错误的输入
	if ip == nil {
		return -1
	}
	if ip.To4() == nil {
		return 1
	} else {
		return 0
	}
}

func saveClientBasicInfo(info map[string]interface{}, uuid string, fallbackIP string) error {
	allowed := map[string]interface{}{}
	stringFields := map[string]int{
		"cpu_name": 100, "virtualization": 50, "arch": 50, "os": 100,
		"kernel_version": 100, "gpu_name": 100, "ipv4": 100, "ipv6": 100, "version": 100,
	}
	for key, maxLen := range stringFields {
		if value, ok := info[key]; ok {
			text, ok := value.(string)
			if !ok || len(text) > maxLen {
				return fmt.Errorf("invalid basic info field %s", key)
			}
			allowed[key] = strings.TrimSpace(text)
		}
	}
	for _, key := range []string{"cpu_cores", "cpu_physical_cores", "mem_total", "swap_total", "disk_total"} {
		if value, ok := info[key]; ok {
			number, ok := numericBasicInfo(value)
			if !ok || number < 0 {
				return fmt.Errorf("invalid basic info field %s", key)
			}
			allowed[key] = number
		}
	}
	allowed["uuid"] = uuid
	applyFallbackClientIP(allowed, fallbackIP)
	appendClientRegionFromGeoIP(allowed)
	return clients.SaveClientInfo(allowed)
}

func numericBasicInfo(value interface{}) (int64, bool) {
	switch number := value.(type) {
	case float64:
		if number != float64(int64(number)) || number > float64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(number), true
	case int:
		return int64(number), true
	case int64:
		return number, true
	default:
		return 0, false
	}
}

func applyFallbackClientIP(info map[string]interface{}, fallbackIP string) {
	if hasClientIP(info) {
		return
	}
	ip := net.ParseIP(fallbackIP)

	switch getClientIPType(ip) {
	case 0:
		info["ipv4"] = fallbackIP
	case 1:
		info["ipv6"] = fallbackIP
	}
}

func hasClientIP(info map[string]interface{}) bool {
	if ipv4, ok := info["ipv4"].(string); ok && ipv4 != "" {
		return true
	}
	if ipv6, ok := info["ipv6"].(string); ok && ipv6 != "" {
		return true
	}
	return false
}

func appendClientRegionFromGeoIP(info map[string]interface{}) {
	cfg, err := config.GetAs[bool](config.GeoIpEnabledKey)
	if err != nil || !cfg {
		return
	}

	for _, key := range []string{"ipv4", "ipv6"} {
		ipStr, ok := info[key].(string)
		if !ok || ipStr == "" {
			continue
		}
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		record, _ := geoip.GetGeoInfo(ip)
		if record == nil {
			continue
		}
		region := geoip.GetRegionUnicodeEmoji(record.ISOCode)
		if region == "" {
			continue
		}
		info["region"] = region
		return
	}
}
