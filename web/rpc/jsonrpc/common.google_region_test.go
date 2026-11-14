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
