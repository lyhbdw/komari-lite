package flags_pkg

import "testing"

func TestConfigValidateRejectsInvalidStartupSecuritySettings(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"missing endpoint", Config{Token: "token", Interval: 1, MaxRetries: 1}},
		{"endpoint without scheme", Config{Endpoint: "panel.example", Token: "token", Interval: 1, MaxRetries: 1}},
		{"missing token", Config{Endpoint: "https://panel.example", Interval: 1, MaxRetries: 1}},
		{"zero interval", Config{Endpoint: "https://panel.example", Token: "token", Interval: 0, MaxRetries: 1}},
		{"zero max retries", Config{Endpoint: "https://panel.example", Token: "token", Interval: 1, MaxRetries: 0}},
		{"invalid month rotate", Config{Endpoint: "https://panel.example", Token: "token", Interval: 1, MaxRetries: 1, MonthRotate: 32}},
		{"invalid custom ipv4", Config{Endpoint: "https://panel.example", Token: "token", Interval: 1, MaxRetries: 1, CustomIpv4: "not-an-ip"}},
		{"invalid custom ipv6", Config{Endpoint: "https://panel.example", Token: "token", Interval: 1, MaxRetries: 1, CustomIpv6: "127.0.0.1"}},
		{"invalid custom dns", Config{Endpoint: "https://panel.example", Token: "token", Interval: 1, MaxRetries: 1, CustomDNS: "8.8.8.8,not-an-ip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatalf("Validate() accepted invalid config: %+v", tt.cfg)
			}
		})
	}
}

func TestConfigValidateAcceptsValidStartupSecuritySettings(t *testing.T) {
	cfg := Config{
		Endpoint:           "https://panel.example/base",
		Token:              "token",
		Interval:           1,
		MaxRetries:         1,
		ReconnectInterval:  1,
		InfoReportInterval: 1,
		MonthRotate:        31,
		CustomIpv4:         "192.0.2.10",
		CustomIpv6:         "2001:db8::10",
		CustomDNS:          "8.8.8.8,2001:4860:4860::8888",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected valid config: %v", err)
	}
}
