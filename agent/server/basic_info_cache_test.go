package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	monitoring "github.com/lyhbdw/komari-lite/agent/monitoring/unit"
)

func TestBasicInfoConcurrentUploadsAreDeduplicated(t *testing.T) {
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer s.Close()
	saved := *flags
	flags.Endpoint = s.URL
	flags.Token = "test-token"
	flags.InfoReportInterval = 5
	flags.CustomIpv4 = "192.0.2.1"
	flags.CustomIpv6 = "2001:db8::1"
	monitoring.InvalidateIPCache()
	t.Cleanup(func() { *flags = saved; monitoring.InvalidateIPCache() })
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := uploadBasicInfo(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("basic info requests=%d; want 1", n)
	}
}

func TestBasicInfoUploadIncludesGoogleRegionChecks(t *testing.T) {
	var got map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var envelope struct {
			Params struct {
				Info map[string]json.RawMessage `json:"info"`
			} `json:"params"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		got = envelope.Params.Info
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	savedFlags := *flags
	flags.Endpoint = server.URL
	flags.Token = "region-test-token"
	flags.DisableCompression = true
	flags.InfoReportInterval = 5
	flags.CustomIpv4 = "192.0.2.1"
	flags.CustomIpv6 = "2001:db8::1"
	monitoring.InvalidateIPCache()
	previousChecks := googleRegionState.checks
	basicInfoUpload.Lock()
	previousPayload := basicInfoUpload.lastPayload
	previousSentAt := basicInfoUpload.sentAt
	basicInfoUpload.lastPayload = ""
	basicInfoUpload.Unlock()
	now := time.Date(2026, 11, 15, 1, 0, 0, 0, time.UTC)
	googleRegionState.Lock()
	googleRegionState.checks = googleRegionChecks{
		IPv4: regionFamilyChecks{
			GoogleAccounts: regionProbeResult{Country: "CN", Status: regionProbeStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTube:        regionProbeResult{Country: "US", Status: regionProbeStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTubePremium: regionProbeResult{Country: "CN", Status: regionProbeStatusOK, CheckedAt: &now, DetectedAt: &now},
		},
		IPv6: regionFamilyChecks{
			GoogleAccounts: regionProbeResult{Status: regionProbeStatusUnknown, Reason: "timeout", CheckedAt: &now},
			YouTube:        regionProbeResult{Country: "US", Status: regionProbeStatusOK, CheckedAt: &now, DetectedAt: &now},
			YouTubePremium: regionProbeResult{Country: "US", Status: regionProbeStatusOK, CheckedAt: &now, DetectedAt: &now},
		},
		CheckedAt: &now,
	}
	googleRegionState.Unlock()
	t.Cleanup(func() {
		*flags = savedFlags
		monitoring.InvalidateIPCache()
		googleRegionState.Lock()
		googleRegionState.checks = previousChecks
		googleRegionState.Unlock()
		basicInfoUpload.Lock()
		basicInfoUpload.lastPayload = previousPayload
		basicInfoUpload.sentAt = previousSentAt
		basicInfoUpload.Unlock()
	})

	if err := uploadBasicInfo(); err != nil {
		t.Fatalf("upload basic info: %v", err)
	}
	if got["google_regions"] == nil {
		t.Fatalf("basic info upload omitted google_regions: %+v", got)
	}
	var decoded googleRegionChecks
	if err := json.Unmarshal(got["google_regions"], &decoded); err != nil {
		t.Fatalf("decode uploaded google_regions: %v", err)
	}
	if decoded.IPv4.GoogleAccounts.Country != "CN" || decoded.IPv4.YouTube.Country != "US" || decoded.IPv6.GoogleAccounts.Status != regionProbeStatusUnknown {
		t.Fatalf("uploaded region checks mismatch: %+v", decoded)
	}
}
