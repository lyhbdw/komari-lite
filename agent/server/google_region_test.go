package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseGoogleAccountsRegion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{name: "US", status: http.StatusOK, body: `<input type="hidden" name="region" value="US">`, want: "US"},
		{name: "CN", status: http.StatusOK, body: `<input type="hidden" name="region" value="CN">`, want: "CN"},
		{name: "challenge", status: http.StatusOK, body: `<html>unusual traffic</html>`, wantErr: true},
		{name: "missing marker", status: http.StatusOK, body: `<html>Sign in</html>`, wantErr: true},
		{name: "invalid code", status: http.StatusOK, body: `<input name="region" value="USA">`, wantErr: true},
		{name: "http failure", status: http.StatusServiceUnavailable, body: `<input name="region" value="US">`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGoogleAccountsRegion(tc.status, "https://accounts.google.com/", []byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseGoogleAccountsRegion() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("parseGoogleAccountsRegion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseYouTubePremiumRegion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{name: "US", status: http.StatusOK, body: `{"contentRegion":"US"}`, want: "US"},
		{name: "CN", status: http.StatusOK, body: `{"contentRegion":"CN"}`, want: "CN"},
		{name: "challenge", status: http.StatusOK, body: `unusual traffic /sorry/`, wantErr: true},
		{name: "missing marker", status: http.StatusOK, body: `<html>YouTube Premium</html>`, wantErr: true},
		{name: "invalid code", status: http.StatusOK, body: `{"contentRegion":"USA"}`, wantErr: true},
		{name: "http failure", status: http.StatusForbidden, body: `{"contentRegion":"US"}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseYouTubePremiumRegion(tc.status, "https://www.youtube.com/premium", []byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseYouTubePremiumRegion() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("parseYouTubePremiumRegion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseYouTubeHomeRegion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "anti XSSI", body: `)]}'` + "\n" + `[["yt.sw.adr",null,[[["en","US"]]]]]`, want: "US"},
		{name: "CN", body: `[["yt.sw.adr",null,[[["en","CN"]]]]]`, want: "CN"},
		{name: "missing marker", body: `<html>challenge</html>`, wantErr: true},
		{name: "invalid country", body: `[["yt.sw.adr",null,[[["en","USA"]]]]]`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseYouTubeHomeRegion(http.StatusOK, "https://www.youtube.com/sw.js_data", []byte(tc.body))
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("parseYouTubeHomeRegion() = %q, %v; want %q, wantErr %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestFetchGoogleRegionForcesAddressFamilyAndSendsNoCookies(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		t.Run(network, func(t *testing.T) {
			t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
			host := "127.0.0.1:0"
			if network == "tcp6" {
				host = "[::1]:0"
			}
			listener, err := net.Listen(network, host)
			if err != nil {
				t.Skipf("address family unavailable: %v", err)
			}
			defer listener.Close()

			var remoteAddr, cookie string
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				remoteAddr = r.RemoteAddr
				cookie = r.Header.Get("Cookie")
				w.Write([]byte(`<input name="region" value="US">`))
			}))
			server.Listener = listener
			server.Start()
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			got, err := fetchGoogleRegion(ctx, network, server.URL, parseGoogleAccountsRegion)
			if err != nil {
				t.Fatal(err)
			}
			if got != "US" {
				t.Fatalf("fetchGoogleRegion() = %q, want US", got)
			}
			if cookie != "" {
				t.Fatalf("probe sent shared cookies: %q", cookie)
			}
			if network == "tcp6" && !strings.HasPrefix(remoteAddr, "[") {
				t.Fatalf("tcp6 request came from %q", remoteAddr)
			}
			if network == "tcp4" && strings.HasPrefix(remoteAddr, "[") {
				t.Fatalf("tcp4 request came from %q", remoteAddr)
			}
		})
	}
}

func TestFailedGoogleRegionProbeKeepsLastCountryButReportsUnknown(t *testing.T) {
	previous := regionProbeResult{
		Country:    "US",
		Status:     regionProbeStatusOK,
		CheckedAt:  timePointer(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)),
		DetectedAt: timePointer(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)),
	}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	got := updateRegionProbeResult(previous, "", context.DeadlineExceeded, now)
	if got.Status != regionProbeStatusUnknown || got.Country != "US" {
		t.Fatalf("failed result = %+v, want unknown status with last country US", got)
	}
	if got.CheckedAt == nil || !got.CheckedAt.Equal(now) || got.DetectedAt == nil || !got.DetectedAt.Equal(*previous.DetectedAt) {
		t.Fatalf("failed timestamps = checked %v detected %v", got.CheckedAt, got.DetectedAt)
	}
}

func timePointer(value time.Time) *time.Time { return &value }

func TestParseYouTubeHomeRegionUsesServiceLocaleField(t *testing.T) {
	body := `)]}'` + "\n" + `[["yt.sw.adr",null,[[["en","US",null,"203.0.113.1"]]]]]`
	got, err := parseYouTubeHomeRegion(http.StatusOK, "https://www.youtube.com/sw.js_data", []byte(body))
	if err != nil || got != "US" {
		t.Fatalf("parseYouTubeHomeRegion() = %q, %v; want US", got, err)
	}
}

func TestYouTubePremiumRerouteNeedsSuccessfulGoogleCNResponse(t *testing.T) {
	if got, err := parseYouTubePremiumRegion(http.StatusOK, "https://www.google.cn/", nil); err != nil || got != "CN" {
		t.Fatalf("successful google.cn response = %q, %v; want CN", got, err)
	}
	if got, err := parseYouTubePremiumRegion(http.StatusFound, "https://www.google.cn/", nil); err == nil || got != "" {
		t.Fatalf("unsuccessful google.cn response = %q, %v; want unknown", got, err)
	}
}
