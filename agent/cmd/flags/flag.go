package flags_pkg

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Config struct {
	Token               string  `json:"token" env:"AGENT_TOKEN"`
	Endpoint            string  `json:"endpoint" env:"AGENT_ENDPOINT"`
	Interval            float64 `json:"interval" env:"AGENT_INTERVAL"`
	IgnoreUnsafeCert    bool    `json:"ignore_unsafe_cert" env:"AGENT_IGNORE_UNSAFE_CERT"`
	MaxRetries          int     `json:"max_retries" env:"AGENT_MAX_RETRIES"`
	ReconnectInterval   int     `json:"reconnect_interval" env:"AGENT_RECONNECT_INTERVAL"`
	InfoReportInterval  int     `json:"info_report_interval" env:"AGENT_INFO_REPORT_INTERVAL"`
	IncludeNics         string  `json:"include_nics" env:"AGENT_INCLUDE_NICS"`
	ExcludeNics         string  `json:"exclude_nics" env:"AGENT_EXCLUDE_NICS"`
	IncludeMountpoints  string  `json:"include_mountpoints" env:"AGENT_INCLUDE_MOUNTPOINTS"`
	MonthRotate         int     `json:"month_rotate" env:"AGENT_MONTH_ROTATE"`
	MemoryIncludeCache  bool    `json:"memory_include_cache" env:"AGENT_MEMORY_INCLUDE_CACHE"`
	MemoryReportRawUsed bool    `json:"memory_report_raw_used" env:"AGENT_MEMORY_REPORT_RAW_USED"`
	CustomDNS           string  `json:"custom_dns" env:"AGENT_CUSTOM_DNS"`
	EnableGPU           bool    `json:"enable_gpu" env:"AGENT_ENABLE_GPU"`
	CustomIpv4          string  `json:"custom_ipv4" env:"AGENT_CUSTOM_IPV4"`
	CustomIpv6          string  `json:"custom_ipv6" env:"AGENT_CUSTOM_IPV6"`
	GetIpAddrFromNic    bool    `json:"get_ip_addr_from_nic" env:"AGENT_GET_IP_ADDR_FROM_NIC"`
	HostProc            string  `json:"host_proc" env:"HOST_PROC"`
	ConfigFile          string  `json:"config_file" env:"AGENT_CONFIG_FILE"`
	DisableCompression  bool    `json:"disable_compression" env:"AGENT_DISABLE_COMPRESSION"`
	PreferIPVersion     string  `json:"prefer_ip_version" env:"AGENT_PREFER_IP_VERSION"`
	MigrationReadyFile  string  `json:"migration_ready_file" env:"AGENT_MIGRATION_READY_FILE"`
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return fmt.Errorf("endpoint is required")
	}
	u, err := url.ParseRequestURI(c.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return fmt.Errorf("endpoint must be an http or https URL with a host")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if c.MaxRetries <= 0 {
		return fmt.Errorf("max-retries must be positive")
	}
	if c.ReconnectInterval <= 0 {
		return fmt.Errorf("reconnect-interval must be positive")
	}
	if c.InfoReportInterval <= 0 {
		return fmt.Errorf("info-report-interval must be positive")
	}
	if c.MonthRotate < 0 || c.MonthRotate > 31 {
		return fmt.Errorf("month-rotate must be 0 through 31")
	}
	if c.CustomIpv4 != "" {
		ip := net.ParseIP(strings.TrimSpace(c.CustomIpv4))
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("custom-ipv4 must be a valid IPv4 address")
		}
	}
	if c.CustomIpv6 != "" {
		ip := net.ParseIP(strings.TrimSpace(c.CustomIpv6))
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("custom-ipv6 must be a valid IPv6 address")
		}
	}
	if c.CustomDNS != "" {
		for _, server := range strings.Split(c.CustomDNS, ",") {
			ip := net.ParseIP(strings.TrimSpace(server))
			if ip == nil {
				return fmt.Errorf("custom-dns must contain only valid IP addresses")
			}
		}
	}
	return nil
}

var GlobalConfig = &Config{}
