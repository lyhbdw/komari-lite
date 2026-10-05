package geoip_test

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/lyhbdw/komari-monitor-lite/utils/geoip"
)

// 测试GeoIP数据库的初始化和更新功能
func TestMmdb(t *testing.T) {
	orig := geoip.GeoIpFilePath
	geoip.GeoIpFilePath = filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	defer func() { geoip.GeoIpFilePath = orig }()
	geoip.CurrentProvider, _ = geoip.NewMaxMindGeoIPService()
	testIpAddr(t)
}
func TestIpApi(t *testing.T) {
	geoip.CurrentProvider, _ = geoip.NewIPAPIService()
	testIpAddr(t)
}

func TestGeojs(t *testing.T) {
	geoip.CurrentProvider, _ = geoip.NewGeoJSService()
	testIpAddr(t)
}

func TestIpInfo(t *testing.T) {
	geoip.CurrentProvider, _ = geoip.NewIPInfoService()
	testIpAddr(t)
}
func testIpAddr(t *testing.T) {
	// IPv4
	ipaddr := "8.8.8.8"
	ip := net.ParseIP(ipaddr)
	record, err := geoip.GetGeoInfo(ip)
	if err != nil {
		t.Logf("Failed to get GeoIP info for IP %s: %v", ipaddr, err)
	}

	if record != nil {
		if record.ISOCode == "" && record.Name == "" {
			t.Errorf("Country information is missing for IP %s", ipaddr)
		}
		t.Logf("IPv4:[%s]%s - %s", ipaddr, record.ISOCode, record.Name)
	} else {
		t.Logf("GeoIP record is nil for IP %s (possibly rate limited: %v)", ipaddr, err)
	}

	// IPv6
	ipaddr = "2001:4860:4860::8888"
	ip = net.ParseIP(ipaddr)
	record, err = geoip.GetGeoInfo(ip)
	if err != nil {
		t.Logf("GetGeoInfo IPv6 %s returned error: %v", ipaddr, err)
	}
	if record != nil {
		if record.ISOCode == "" && record.Name == "" {
			t.Errorf("Country information is missing for IPv6 %s", ipaddr)
		}
		t.Logf("IPv6:[%s]%s - %s", ipaddr, record.ISOCode, record.Name)
	} else {
		t.Logf("GeoIP record is nil for IPv6 %s", ipaddr)
	}
}

func TestUnicodeEmoji(t *testing.T) {
	ISOCode := "CN"
	emoji := geoip.GetRegionUnicodeEmoji(ISOCode)
	if emoji != "🇨🇳" {
		t.Errorf("Expected emoji for %s, got %s", ISOCode, emoji)
	}
	t.Logf("Emoji for %s: %s", ISOCode, emoji)
}
