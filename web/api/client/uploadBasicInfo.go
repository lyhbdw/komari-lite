package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/utils/geoip"
	"io"
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
	if value, ok := info["google_regions"]; ok {
		regions, err := decodeGoogleRegionChecks(value)
		if err != nil {
			return err
		}
		if current, err := clients.GetClientByUUID(uuid); err == nil && current.GoogleRegions != "" {
			var previous models.GoogleRegionChecks
			if json.Unmarshal([]byte(current.GoogleRegions), &previous) == nil {
				preserveLastSuccessfulGoogleRegions(&regions, previous)
			}
		}
		encoded, err := json.Marshal(regions)
		if err != nil {
			return fmt.Errorf("encode google_regions: %w", err)
		}
		allowed["google_regions"] = string(encoded)
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

func preserveLastSuccessfulGoogleRegions(current *models.GoogleRegionChecks, previous models.GoogleRegionChecks) {
	merge := func(next *models.GoogleRegionProbe, old models.GoogleRegionProbe) {
		if next.Status == models.GoogleRegionStatusUnknown && next.Country == "" && validGoogleCountryCode(old.Country) {
			next.Country = old.Country
			next.DetectedAt = old.DetectedAt
		}
	}
	merge(&current.IPv4.GoogleAccounts, previous.IPv4.GoogleAccounts)
	merge(&current.IPv4.YouTube, previous.IPv4.YouTube)
	merge(&current.IPv4.YouTubePremium, previous.IPv4.YouTubePremium)
	merge(&current.IPv6.GoogleAccounts, previous.IPv6.GoogleAccounts)
	merge(&current.IPv6.YouTube, previous.IPv6.YouTube)
	merge(&current.IPv6.YouTubePremium, previous.IPv6.YouTubePremium)
}

func decodeGoogleRegionChecks(value interface{}) (models.GoogleRegionChecks, error) {
	var encoded []byte
	switch typed := value.(type) {
	case json.RawMessage:
		encoded = typed
	case []byte:
		encoded = typed
	default:
		var err error
		encoded, err = json.Marshal(value)
		if err != nil {
			return models.GoogleRegionChecks{}, fmt.Errorf("invalid google_regions: %w", err)
		}
	}
	if len(encoded) == 0 || len(encoded) > 8192 {
		return models.GoogleRegionChecks{}, fmt.Errorf("invalid google_regions size")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var regions models.GoogleRegionChecks
	if err := decoder.Decode(&regions); err != nil {
		return models.GoogleRegionChecks{}, fmt.Errorf("invalid google_regions: %w", err)
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return models.GoogleRegionChecks{}, fmt.Errorf("invalid google_regions: trailing data")
	}
	if regions.CheckedAt == nil {
		return models.GoogleRegionChecks{}, fmt.Errorf("invalid google_regions: checked_at is required")
	}
	for family, checks := range map[string]models.GoogleRegionFamilyChecks{"ipv4": regions.IPv4, "ipv6": regions.IPv6} {
		if err := validateGoogleRegionProbe(family+".google_accounts", checks.GoogleAccounts); err != nil {
			return models.GoogleRegionChecks{}, err
		}
		if err := validateGoogleRegionProbe(family+".youtube", checks.YouTube); err != nil {
			return models.GoogleRegionChecks{}, err
		}
		if err := validateGoogleRegionProbe(family+".youtube_premium", checks.YouTubePremium); err != nil {
			return models.GoogleRegionChecks{}, err
		}
	}
	return regions, nil
}

func validateGoogleRegionProbe(name string, probe models.GoogleRegionProbe) error {
	if probe.CheckedAt == nil {
		return fmt.Errorf("invalid google_regions: %s.checked_at is required", name)
	}
	switch probe.Status {
	case models.GoogleRegionStatusOK:
		if !validGoogleCountryCode(probe.Country) || probe.DetectedAt == nil || probe.Reason != "" {
			return fmt.Errorf("invalid google_regions: %s has an invalid successful result", name)
		}
	case models.GoogleRegionStatusUnknown:
		if probe.Reason == "" || len(probe.Reason) > 64 || !validGoogleRegionReason(probe.Reason) {
			return fmt.Errorf("invalid google_regions: %s has an invalid unknown reason", name)
		}
		if probe.Country != "" && !validGoogleCountryCode(probe.Country) {
			return fmt.Errorf("invalid google_regions: %s has an invalid last-known country", name)
		}
	default:
		return fmt.Errorf("invalid google_regions: %s has an invalid status", name)
	}
	return nil
}

func validGoogleCountryCode(country string) bool {
	return len(country) == 2 && country[0] >= 'A' && country[0] <= 'Z' && country[1] >= 'A' && country[1] <= 'Z'
}

func validGoogleRegionReason(reason string) bool {
	if strings.HasPrefix(reason, "http_") {
		code := strings.TrimPrefix(reason, "http_")
		return len(code) == 3 && code[0] >= '1' && code[0] <= '5' && code[1] >= '0' && code[1] <= '9' && code[2] >= '0' && code[2] <= '9'
	}
	switch reason {
	case "not_checked", "captcha", "marker_missing", "marker_invalid", "rate_limited", "blocked", "timeout", "network_error", "response_too_large", "proxy_egress_unknown":
		return true
	default:
		return false
	}
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
	if !hasClientIP(info) {
		return
	}
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
