package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	flags_pkg "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
)

const (
	googleRegionCheckInterval = 6 * time.Hour
	googleRegionCheckTimeout  = 10 * time.Second
	googleRegionMaxBodyBytes  = 2 << 20
	regionProbeStatusOK       = "ok"
	regionProbeStatusUnknown  = "unknown"
)

const (
	googleAccountsRegionEndpoint = "https://accounts.google.com/v3/signin/identifier?flowName=GlifSetupAndroid"
	youtubeHomeRegionEndpoint    = "https://www.youtube.com/sw.js_data"
	youtubePremiumEndpoint       = "https://www.youtube.com/premium"
	regionProbeUserAgent         = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
)

var googleAccountsRegionPattern = regexp.MustCompile(`\bname="region"\s+value="([A-Za-z]{2})"`)
var youtubeContentRegionPattern = regexp.MustCompile(`"contentRegion"\s*:\s*"([A-Za-z]{2})"`)

type regionResponseParser func(int, string, []byte) (string, error)

type regionProbeResult struct {
	Country    string     `json:"country,omitempty"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason,omitempty"`
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
	DetectedAt *time.Time `json:"detected_at,omitempty"`
}

type regionFamilyChecks struct {
	GoogleAccounts regionProbeResult `json:"google_accounts"`
	YouTube        regionProbeResult `json:"youtube"`
	YouTubePremium regionProbeResult `json:"youtube_premium"`
}

type googleRegionChecks struct {
	IPv4      regionFamilyChecks `json:"ipv4"`
	IPv6      regionFamilyChecks `json:"ipv6"`
	CheckedAt *time.Time         `json:"checked_at,omitempty"`
}

var googleRegionState struct {
	sync.RWMutex
	checks  googleRegionChecks
	running bool
}

func currentGoogleRegionChecks() googleRegionChecks {
	googleRegionState.RLock()
	checks := googleRegionState.checks
	googleRegionState.RUnlock()
	if checks.IPv4.GoogleAccounts.Status == "" {
		return initialGoogleRegionChecks()
	}
	return checks
}

func parseGoogleAccountsRegion(statusCode int, _ string, body []byte) (string, error) {
	if statusCode != http.StatusOK {
		return "", regionHTTPError(statusCode)
	}
	if containsRegionChallenge(body) {
		return "", errors.New("captcha")
	}
	match := googleAccountsRegionPattern.FindSubmatch(body)
	if len(match) != 2 {
		return "", errors.New("marker_missing")
	}
	return strings.ToUpper(string(match[1])), nil
}

func parseYouTubeHomeRegion(statusCode int, _ string, body []byte) (string, error) {
	if statusCode != http.StatusOK {
		return "", regionHTTPError(statusCode)
	}
	if containsRegionChallenge(body) {
		return "", errors.New("captcha")
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(body), ")]}'"))
	var response []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &response); err != nil || len(response) == 0 {
		return "", errors.New("marker_missing")
	}
	var row []json.RawMessage
	if err := json.Unmarshal(response[0], &row); err != nil || len(row) < 3 {
		return "", errors.New("marker_missing")
	}
	var name string
	if json.Unmarshal(row[0], &name) != nil || name != "yt.sw.adr" {
		return "", errors.New("marker_missing")
	}
	var groups []json.RawMessage
	if err := json.Unmarshal(row[2], &groups); err != nil || len(groups) == 0 {
		return "", errors.New("marker_missing")
	}
	var firstGroup []json.RawMessage
	if err := json.Unmarshal(groups[0], &firstGroup); err != nil || len(firstGroup) == 0 {
		return "", errors.New("marker_missing")
	}
	var locale []json.RawMessage
	if err := json.Unmarshal(firstGroup[0], &locale); err != nil || len(locale) < 2 {
		return "", errors.New("marker_missing")
	}
	var country string
	if json.Unmarshal(locale[1], &country) != nil || !validCountryCode(country) {
		return "", errors.New("marker_invalid")
	}
	return strings.ToUpper(country), nil
}

func parseYouTubePremiumRegion(statusCode int, finalURL string, body []byte) (string, error) {
	parsedURL, err := url.Parse(finalURL)
	if statusCode != http.StatusOK {
		return "", regionHTTPError(statusCode)
	}
	if containsRegionChallenge(body) {
		return "", errors.New("captcha")
	}
	if err == nil && strings.EqualFold(parsedURL.Hostname(), "www.google.cn") {
		return "CN", nil
	}
	match := youtubeContentRegionPattern.FindSubmatch(body)
	if len(match) != 2 {
		return "", errors.New("marker_missing")
	}
	country := strings.ToUpper(string(match[1]))
	if !validCountryCode(country) {
		return "", errors.New("marker_invalid")
	}
	return country, nil
}

func containsRegionChallenge(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "unusual traffic") ||
		strings.Contains(text, "/sorry/") ||
		strings.Contains(text, "g-recaptcha") ||
		strings.Contains(text, "before you continue") ||
		strings.Contains(text, "consent.google.com") ||
		strings.Contains(text, "consent.youtube.com")
}

func validCountryCode(country string) bool {
	if len(country) != 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if country[i] < 'A' || country[i] > 'Z' {
			if country[i] < 'a' || country[i] > 'z' {
				return false
			}
		}
	}
	return true
}

func regionHTTPError(status int) error {
	switch status {
	case http.StatusTooManyRequests:
		return errors.New("rate_limited")
	case http.StatusForbidden, http.StatusUnauthorized:
		return errors.New("blocked")
	default:
		return fmt.Errorf("http_%d", status)
	}
}

func regionFailureReason(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch {
	case message == "captcha":
		return "captcha"
	case message == "marker_missing" || message == "marker_invalid":
		return message
	case message == "rate_limited":
		return message
	case message == "blocked":
		return message
	case strings.HasPrefix(message, "http_"):
		return message
	case message == "proxy_egress_unknown":
		return message
	case message == "response_too_large":
		return message
	case message == "network unreachable":
		return "network_error"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "network_error"
	}
}

func updateRegionProbeResult(previous regionProbeResult, country string, probeErr error, now time.Time) regionProbeResult {
	now = now.UTC()
	previous.CheckedAt = &now
	if probeErr != nil || !validCountryCode(country) {
		previous.Status = regionProbeStatusUnknown
		previous.Reason = regionFailureReason(probeErr)
		if probeErr == nil {
			previous.Reason = "marker_invalid"
		}
		if previous.Country == "" {
			previous.DetectedAt = nil
		}
		return previous
	}
	country = strings.ToUpper(country)
	if previous.Country != country || previous.DetectedAt == nil {
		previous.DetectedAt = &now
	}
	previous.Country = country
	previous.Status = regionProbeStatusOK
	previous.Reason = ""
	return previous
}

func fetchGoogleRegion(ctx context.Context, network, endpoint string, parser regionResponseParser) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", regionProbeUserAgent)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")

	transport := &http.Transport{
		Proxy: nil,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: flags_pkg.GlobalConfig.IgnoreUnsafeCert,
		},
		DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
			return dnsresolver.GetNetDialer(5*time.Second).DialContext(ctx, network, address)
		},
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   googleRegionCheckTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	defer transport.CloseIdleConnections()

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, googleRegionMaxBodyBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > googleRegionMaxBodyBytes {
		return "", errors.New("response_too_large")
	}
	return parser(response.StatusCode, response.Request.URL.String(), body)
}

func probeRegionFamily(ipVersion string, previous regionFamilyChecks, now time.Time) regionFamilyChecks {
	network := "tcp" + ipVersion
	probes := []struct {
		endpoint string
		parser   regionResponseParser
		previous regionProbeResult
	}{
		{googleAccountsRegionEndpoint, parseGoogleAccountsRegion, previous.GoogleAccounts},
		{youtubeHomeRegionEndpoint, parseYouTubeHomeRegion, previous.YouTube},
		{youtubePremiumEndpoint, parseYouTubePremiumRegion, previous.YouTubePremium},
	}
	results := make([]regionProbeResult, len(probes))
	var wg sync.WaitGroup
	for index, item := range probes {
		index, item := index, item
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), googleRegionCheckTimeout)
			country, err := fetchGoogleRegion(ctx, network, item.endpoint, item.parser)
			cancel()
			results[index] = updateRegionProbeResult(item.previous, country, err, now)
		}()
	}
	wg.Wait()
	previous.GoogleAccounts = results[0]
	previous.YouTube = results[1]
	previous.YouTubePremium = results[2]
	return previous
}

func runGoogleRegionChecks() {
	googleRegionState.Lock()
	if googleRegionState.running {
		googleRegionState.Unlock()
		return
	}
	googleRegionState.running = true
	previous := currentGoogleRegionChecksLocked()
	if previous.CheckedAt == nil {
		previous.IPv4 = initialGoogleRegionChecks().IPv4
		previous.IPv6 = initialGoogleRegionChecks().IPv6
	}
	googleRegionState.Unlock()
	defer func() {
		googleRegionState.Lock()
		googleRegionState.running = false
		googleRegionState.Unlock()
	}()

	if previous.CheckedAt != nil && time.Since(*previous.CheckedAt) < googleRegionCheckInterval {
		return
	}

	now := time.Now().UTC()
	var wg sync.WaitGroup
	var next googleRegionChecks
	var resultMu sync.Mutex
	for _, family := range []string{"4", "6"} {
		family := family
		wg.Add(1)
		go func() {
			defer wg.Done()
			old := previous.IPv4
			if family == "6" {
				old = previous.IPv6
			}
			got := probeRegionFamily(family, old, now)
			resultMu.Lock()
			if family == "4" {
				next.IPv4 = got
			} else {
				next.IPv6 = got
			}
			resultMu.Unlock()
		}()
	}
	wg.Wait()
	next.CheckedAt = &now
	googleRegionState.Lock()
	googleRegionState.checks = next
	googleRegionState.Unlock()
	UpdateBasicInfo()
}

func currentGoogleRegionChecksLocked() googleRegionChecks {
	if googleRegionState.checks.IPv4.GoogleAccounts.Status == "" {
		return initialGoogleRegionChecks()
	}
	return googleRegionState.checks
}

func initialGoogleRegionChecks() googleRegionChecks {
	unknown := func() regionProbeResult {
		return regionProbeResult{Status: regionProbeStatusUnknown, Reason: "not_checked"}
	}
	family := func() regionFamilyChecks {
		return regionFamilyChecks{GoogleAccounts: unknown(), YouTube: unknown(), YouTubePremium: unknown()}
	}
	return googleRegionChecks{IPv4: family(), IPv6: family()}
}

func googleRegionChecksDue(now time.Time) bool {
	googleRegionState.RLock()
	checkedAt := googleRegionState.checks.CheckedAt
	running := googleRegionState.running
	googleRegionState.RUnlock()
	return !running && (checkedAt == nil || now.Sub(*checkedAt) >= googleRegionCheckInterval)
}

func DoGoogleRegionCheckWorks() {
	if googleRegionChecksDue(time.Now()) {
		go runGoogleRegionChecks()
	}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if googleRegionChecksDue(time.Now()) {
			go runGoogleRegionChecks()
		}
	}
}
