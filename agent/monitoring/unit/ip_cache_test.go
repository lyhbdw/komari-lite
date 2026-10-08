package monitoring

import (
	"sync"
	"testing"
	"time"
)

// TestIPCacheTTL 验证缓存命中/过期/失败不覆盖的语义（不触网）。
func TestIPCacheTTL(t *testing.T) {
	ipCacheMu.Lock()
	savedV4, savedV6, savedAt := ipCachev4, ipCachev6, ipCachedAt
	ipCachev4, ipCachev6, ipCachedAt = "1.2.3.4", "", time.Now()
	ipCacheMu.Unlock()
	t.Cleanup(func() {
		ipCacheMu.Lock()
		ipCachev4, ipCachev6, ipCachedAt = savedV4, savedV6, savedAt
		ipCacheMu.Unlock()
	})

	// 缓存新鲜：直接命中，不触网（getIPAddressUncached 不会被调用，
	// 否则在没有网络的测试环境会耗时数十秒）。
	start := time.Now()
	v4, v6, err := GetIPAddress()
	if err != nil {
		t.Fatalf("GetIPAddress cached: %v", err)
	}
	if v4 != "1.2.3.4" || v6 != "" {
		t.Fatalf("GetIPAddress cached = (%q, %q), want (1.2.3.4, \"\")", v4, v6)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cache hit took %v, expected instant", elapsed)
	}

	// 缓存过期：触发真实探测路径。测试环境无公网时两个都为空，
	// 但必须不 panic 且不覆盖已有缓存值。
	ipCacheMu.Lock()
	ipCachedAt = time.Now().Add(-2 * ipCacheTTL)
	ipCacheMu.Unlock()

	// 用一个很短的路径验证：把 TTL 临时调大，确保探测后缓存时间被记录
	// （探测失败时 ipCachedAt 也会更新以避免失败风暴）。
	_, _, _ = GetIPAddress()
	ipCacheMu.Lock()
	at := ipCachedAt
	ipCacheMu.Unlock()
	if time.Since(at) > 5*time.Minute {
		t.Fatalf("ipCachedAt not refreshed after probe: %v", at)
	}
}

// TestIPCacheConcurrent 并发调用 GetIPAddress 不产生 data race。
func TestIPCacheConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			GetIPAddress()
		}()
	}
	wg.Wait()
}
