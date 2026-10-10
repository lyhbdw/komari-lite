package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

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
