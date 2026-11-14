package client

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/cmd/flags"
	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
	"github.com/lyhbdw/komari-lite/utils/geoip"
)

type staticGeoIPProvider struct {
	name string
	iso  string
}

func (p staticGeoIPProvider) Name() string {
	return p.name
}

func (p staticGeoIPProvider) GetGeoInfo(ip net.IP) (*geoip.GeoInfo, error) {
	return &geoip.GeoInfo{ISOCode: p.iso, Name: p.iso}, nil
}

func (p staticGeoIPProvider) UpdateDatabase() error {
	return nil
}

func (p staticGeoIPProvider) Close() error {
	return nil
}

func TestV2BasicInfoFillsRegionFromGeoIP(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:v2_basic_info_geoip?mode=memory&cache=shared"

	db := dbcore.GetDBInstance()
	if err := config.Set(config.GeoIpEnabledKey, true); err != nil {
		t.Fatalf("enable geoip: %v", err)
	}

	oldProvider := geoip.CurrentProvider
	geoip.CurrentProvider = staticGeoIPProvider{name: t.Name(), iso: "SG"}
	t.Cleanup(func() {
		geoip.CurrentProvider = oldProvider
	})

	clientUUID := "client-v2-geoip"
	now := time.Now().UTC()
	if err := db.Create(&models.Client{
		UUID:      clientUUID,
		Token:     "token-v2-geoip",
		Name:      "client_v2_geoip",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	resp := handleV2RPC(clientUUID, v2.Request{
		JSONRPC: v2.Version,
		Method:  v2.MethodAgentBasicInfo,
		Params: map[string]interface{}{
			"info": map[string]interface{}{
				"ipv4": "8.8.8.8",
			},
		},
		ID: "basic-info",
	}, false, false)
	if resp.Error != nil {
		t.Fatalf("v2 basic info failed: %+v", resp.Error)
	}

	var got models.Client
	if err := db.First(&got, "uuid = ?", clientUUID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	want := geoip.GetRegionUnicodeEmoji("SG")
	if got.Region != want {
		t.Fatalf("expected GeoIP region to be saved, got %q", got.Region)
	}
}

func TestSaveClientBasicInfoPersistsValidatedGoogleRegions(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	clients.InvalidateClientsCache()
	t.Cleanup(clients.InvalidateClientsCache)
	if err := db.Create(&models.Client{UUID: "region-node", Token: "region-token", Name: "region-node"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	regions := models.GoogleRegionChecks{
		IPv4: models.GoogleRegionFamilyChecks{
			GoogleAccounts: models.GoogleRegionProbe{Country: "CN", Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTube:        models.GoogleRegionProbe{Country: "US", Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTubePremium: models.GoogleRegionProbe{Country: "CN", Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now},
		},
		IPv6: models.GoogleRegionFamilyChecks{
			GoogleAccounts: models.GoogleRegionProbe{Status: models.GoogleRegionStatusUnknown, Reason: "timeout", CheckedAt: &now},
			YouTube:        models.GoogleRegionProbe{Country: "US", Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTubePremium: models.GoogleRegionProbe{Country: "US", Status: models.GoogleRegionStatusOK, CheckedAt: &now, DetectedAt: &now},
		},
		CheckedAt: &now,
	}
	encoded, err := json.Marshal(regions)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveClientBasicInfo(map[string]interface{}{"google_regions": json.RawMessage(encoded)}, "region-node", ""); err != nil {
		t.Fatalf("save valid Google region checks: %v", err)
	}
	var stored models.Client
	if err := db.First(&stored, "uuid = ?", "region-node").Error; err != nil {
		t.Fatal(err)
	}
	var got models.GoogleRegionChecks
	if err := json.Unmarshal([]byte(stored.GoogleRegions), &got); err != nil {
		t.Fatalf("decode stored Google region checks %q: %v", stored.GoogleRegions, err)
	}
	if got.IPv4.GoogleAccounts.Country != "CN" || got.IPv4.YouTube.Country != "US" || got.IPv6.GoogleAccounts.Status != models.GoogleRegionStatusUnknown {
		t.Fatalf("stored Google region checks mismatch: %+v", got)
	}

	updatedAt := now.Add(6 * time.Hour)
	got.CheckedAt = &updatedAt
	got.IPv4.GoogleAccounts = models.GoogleRegionProbe{Status: models.GoogleRegionStatusUnknown, Reason: "timeout", CheckedAt: &updatedAt}
	updated, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveClientBasicInfo(map[string]interface{}{"google_regions": json.RawMessage(updated)}, "region-node", ""); err != nil {
		t.Fatalf("save failed refresh: %v", err)
	}
	if err := db.First(&stored, "uuid = ?", "region-node").Error; err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(stored.GoogleRegions), &got); err != nil {
		t.Fatal(err)
	}
	probe := got.IPv4.GoogleAccounts
	if probe.Status != models.GoogleRegionStatusUnknown || probe.Country != "CN" || probe.DetectedAt == nil || !probe.DetectedAt.Equal(now) {
		t.Fatalf("failed refresh discarded last successful country: %+v", probe)
	}
}

func TestSaveClientBasicInfoRejectsInvalidGoogleRegionChecks(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, db)
	clients.InvalidateClientsCache()
	t.Cleanup(clients.InvalidateClientsCache)
	if err := db.Create(&models.Client{UUID: "invalid-region-node", Token: "invalid-region-token", Name: "invalid-region-node"}).Error; err != nil {
		t.Fatal(err)
	}
	invalid := `{"ipv4":{"google_accounts":{"country":"CN","status":"sent_to_china"}}}`
	if err := saveClientBasicInfo(map[string]interface{}{"google_regions": json.RawMessage(invalid)}, "invalid-region-node", ""); err == nil {
		t.Fatal("invalid region status was accepted")
	}
	var stored models.Client
	if err := db.First(&stored, "uuid = ?", "invalid-region-node").Error; err != nil {
		t.Fatal(err)
	}
	if stored.GoogleRegions != "" {
		t.Fatalf("invalid region report changed stored value to %q", stored.GoogleRegions)
	}
}
