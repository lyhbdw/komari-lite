package monitoring

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type ipRoundTripper func(*http.Request) (*http.Response, error)

func (f ipRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func isolatedIPCache(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	old4, old6 := ipv4HTTPClient, ipv6HTTPClient
	oldFlags := *flags
	ipCacheMu.Lock()
	savedFailed := ipFailedAt
	ipFailedAt = time.Time{}
	saved4, saved6, savedAt := ipCachev4, ipCachev6, ipCachedAt
	ipCachev4, ipCachev6, ipCachedAt = "", "", time.Time{}
	ipCacheMu.Unlock()
	flags.GetIpAddrFromNic = false
	flags.CustomIpv4 = ""
	flags.CustomIpv6 = ""
	ipv4HTTPClient = &http.Client{Transport: transport}
	ipv6HTTPClient = ipv4HTTPClient
	t.Cleanup(func() {
		ipv4HTTPClient, ipv6HTTPClient = old4, old6
		*flags = oldFlags
		ipCacheMu.Lock()
		ipCachev4, ipCachev6, ipCachedAt = saved4, saved6, savedAt
		ipFailedAt = savedFailed
		ipCacheMu.Unlock()
	})
}

func TestIPFailureHasNegativeCache(t *testing.T) {
	var calls atomic.Int32
	isolatedIPCache(t, ipRoundTripper(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("offline") }))
	_, _, _ = GetIPAddress()
	before := calls.Load()
	for i := 0; i < 3; i++ {
		_, _, _ = GetIPAddress()
	}
	if before == 0 || calls.Load() != before {
		t.Fatalf("failed probe repeated: %d -> %d", before, calls.Load())
	}
}

func TestIPConcurrentRequestsShareOneProbe(t *testing.T) {
	var calls atomic.Int32
	isolatedIPCache(t, ipRoundTripper(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("offline") }))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = GetIPAddress() }()
	}
	wg.Wait()
	if got := calls.Load(); got != 11 {
		t.Fatalf("provider requests=%d; want one IPv4+IPv6 probe (11)", got)
	}
}

func TestIPCacheTTL(t *testing.T) {
	isolatedIPCache(t, ipRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Error("fresh IP cache hit probed network")
		return nil, errors.New("offline")
	}))
	ipCacheMu.Lock()
	ipCachev4, ipCachev6, ipCachedAt = "192.0.2.1", "", time.Now()
	ipCacheMu.Unlock()
	v4, v6, err := GetIPAddress()
	if err != nil || v4 != "192.0.2.1" || v6 != "" {
		t.Fatalf("cached IP=(%q,%q,%v)", v4, v6, err)
	}
}
