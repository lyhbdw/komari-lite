package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/lyhbdw/komari-lite/database/models"
)

func TestAdminClientItemIncludesGoogleRegionsButPublicModelOmitsThem(t *testing.T) {
	client := models.Client{
		UUID:          "google-region-node",
		GoogleRegions: `{"ipv4":{"google_accounts":{"country":"CN","status":"ok"}}}`,
	}
	encoded, err := json.Marshal(adminClientDetailItem(client, true))
	if err != nil {
		t.Fatal(err)
	}
	var admin map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &admin); err != nil {
		t.Fatal(err)
	}
	var checks models.GoogleRegionChecks
	if err := json.Unmarshal(admin["google_regions"], &checks); err != nil {
		t.Fatalf("admin result missing valid google_regions: %s (%v)", admin["google_regions"], err)
	}

	publicEncoded, err := json.Marshal(client)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(publicEncoded, &public); err != nil {
		t.Fatal(err)
	}
	if _, exists := public["google_regions"]; exists {
		t.Fatal("Google region diagnostics leaked through public Client serialization")
	}

	listEncoded, err := json.Marshal(adminClientItem(client, true))
	if err != nil {
		t.Fatal(err)
	}
	var listItem map[string]json.RawMessage
	if err := json.Unmarshal(listEncoded, &listItem); err != nil {
		t.Fatal(err)
	}
	if _, exists := listItem["google_regions"]; exists {
		t.Fatal("Google region diagnostics leaked through the admin client list")
	}
}
