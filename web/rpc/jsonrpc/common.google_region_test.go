package jsonrpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
)

func TestPublicPossibleChina(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	probe := func(country, status string) models.GoogleRegionProbe {
		result := models.GoogleRegionProbe{Status: status, CheckedAt: &now}
		if country != "" {
			result.Country = country
			result.DetectedAt = &now
		}
		if status == models.GoogleRegionStatusUnknown {
			result.Reason = "timeout"
		}
		return result
	}
	checks := models.GoogleRegionChecks{
		IPv4: models.GoogleRegionFamilyChecks{
			GoogleAccounts: probe("US", models.GoogleRegionStatusOK),
			YouTube:        probe("US", models.GoogleRegionStatusOK),
			YouTubePremium: probe("CN", models.GoogleRegionStatusOK),
		},
		IPv6: models.GoogleRegionFamilyChecks{
			GoogleAccounts: probe("US", models.GoogleRegionStatusOK),
			YouTube:        probe("US", models.GoogleRegionStatusOK),
			YouTubePremium: probe("US", models.GoogleRegionStatusOK),
		},
		CheckedAt: &now,
	}
	encoded, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if got := publicPossibleChina(string(encoded)); got == nil || !*got {
		t.Fatalf("CN signal = %v, want true", got)
	}

	checks.IPv4.YouTubePremium = probe("US", models.GoogleRegionStatusOK)
	encoded, err = json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if got := publicPossibleChina(string(encoded)); got == nil || *got {
		t.Fatalf("all successful non-CN signals = %v, want false", got)
	}

	checks.IPv6.GoogleAccounts = probe("", models.GoogleRegionStatusUnknown)
	encoded, err = json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	if got := publicPossibleChina(string(encoded)); got != nil {
		t.Fatalf("incomplete signals = %v, want unknown", *got)
	}
}

func TestCommonNodeSeparatesAddressFamilies(t *testing.T) {
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	family := func(country string) models.GoogleRegionFamilyChecks {
		probe := models.GoogleRegionProbe{Country: country, Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now}
		return models.GoogleRegionFamilyChecks{GoogleAccounts: probe, YouTube: probe, YouTubePremium: probe}
	}
	failed := family("CN")
	failed.GoogleAccounts.Status = models.GoogleRegionStatusUnknown
	failed.GoogleAccounts.Reason = "timeout"
	failed.YouTube.Country = "US"
	failed.YouTubePremium.Country = "US"
	incomplete := family("US")
	incomplete.YouTubePremium = models.GoogleRegionProbe{Status: models.GoogleRegionStatusUnknown, Reason: "captcha", CheckedAt: &now}
	for _, tc := range []struct {
		name         string
		checks       models.GoogleRegionChecks
		want4, want6 string
		wantCombined string
	}{
		{name: "only IPv4 CN", checks: models.GoogleRegionChecks{IPv4: family("CN"), IPv6: family("US"), CheckedAt: &now}, want4: "true", want6: "false", wantCombined: "true"},
		{name: "only IPv6 CN", checks: models.GoogleRegionChecks{IPv4: family("US"), IPv6: family("CN"), CheckedAt: &now}, want4: "false", want6: "true", wantCombined: "true"},
		{name: "IPv6 unavailable", checks: models.GoogleRegionChecks{IPv4: family("US"), CheckedAt: &now}, want4: "false", want6: "null", wantCombined: "null"},
		{name: "IPv4 unavailable", checks: models.GoogleRegionChecks{IPv6: family("US"), CheckedAt: &now}, want4: "null", want6: "false", wantCombined: "null"},
		{name: "failed current check with old CN", checks: models.GoogleRegionChecks{IPv4: failed, IPv6: family("US"), CheckedAt: &now}, want4: "null", want6: "false", wantCombined: "null"},
		{name: "incomplete IPv6 does not hide IPv4", checks: models.GoogleRegionChecks{IPv4: family("US"), IPv6: incomplete, CheckedAt: &now}, want4: "false", want6: "null", wantCombined: "null"},
		{name: "CN remains decisive within incomplete family", checks: models.GoogleRegionChecks{IPv4: family("CN"), IPv6: incomplete, CheckedAt: &now}, want4: "true", want6: "null", wantCombined: "true"},
		{name: "unchecked", checks: models.GoogleRegionChecks{IPv4: family("US"), IPv6: family("US")}, want4: "null", want6: "null", wantCombined: "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.checks)
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(commonNode(models.Client{UUID: "node", GoogleRegions: string(encoded)}))
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{"possible_china_ipv4": tc.want4, "possible_china_ipv6": tc.want6, "possible_china": tc.wantCombined} {
				if got := string(fields[key]); got != want {
					t.Errorf("%s = %s, want %s", key, got, want)
				}
			}
			for _, private := range []string{"google_regions", "google_accounts", "country", "reason", "checked_at", "detected_at"} {
				if _, exists := fields[private]; exists {
					t.Errorf("private field leaked: %s", private)
				}
			}
		})
	}
}

func TestCommonNodeExposesOnlyDerivedPossibleChina(t *testing.T) {
	encoded := `{"ipv4":{"google_accounts":{"country":"CN","status":"ok"}},"checked_at":"2026-11-15T00:00:00Z"}`
	body, err := json.Marshal(commonNode(models.Client{UUID: "node", GoogleRegions: encoded}))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	var possible bool
	if err := json.Unmarshal(fields["possible_china"], &possible); err != nil || !possible {
		t.Fatalf("possible_china = %s, %v; want true", fields["possible_china"], err)
	}
	if _, exists := fields["google_regions"]; exists {
		t.Fatal("raw Google region checks leaked through common node data")
	}
}
