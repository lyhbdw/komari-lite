package netstatic

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func setupStatic(t *testing.T) {
	t.Helper()
	_ = Stop()
	mu.Lock()
	store = NetStatic{Interfaces: map[string][]TrafficData{}}
	config = configOrDefault(NetStaticConfig{})
	store.Config = config
	staticCache = map[string][]TrafficData{}
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}
	rangeCache = trafficRangeCache{}
	oldPath := SaveFilePath
	SaveFilePath = filepath.Join(t.TempDir(), "net.json")
	mu.Unlock()
	t.Cleanup(func() { _ = Stop(); mu.Lock(); SaveFilePath = oldPath; mu.Unlock() })
}

func TestTrafficRangeIncrementalGrowthAndBackwardsQueries(t *testing.T) {
	setupStatic(t)
	mu.Lock()
	store.Interfaces["eth0"] = []TrafficData{{Timestamp: 99, Tx: 100}, {Timestamp: 100, Tx: 5}, {Timestamp: 110, Tx: 7}}
	mu.Unlock()
	got, _ := GetTotalTrafficBetween(100, 105)
	if got["eth0"].Tx != 5 {
		t.Fatalf("first=%v", got)
	}
	got, _ = GetTotalTrafficBetween(100, 110)
	if got["eth0"].Tx != 12 {
		t.Fatalf("grown=%v", got)
	}
	mu.Lock()
	if rangeCache.builds != 1 {
		t.Errorf("range rebuilt on advancing end: %d", rangeCache.builds)
	}
	appendTrafficLocked("eth0", TrafficData{Timestamp: 111, Tx: 3, Rx: 4})
	mu.Unlock()
	got, _ = GetTotalTrafficBetween(100, 111)
	if got["eth0"].Tx != 15 || got["eth0"].Rx != 4 {
		t.Fatalf("incremental=%v", got)
	}
	got["eth0"] = TrafficData{Tx: 999}
	same, _ := GetTotalTrafficBetween(100, 111)
	if same["eth0"].Tx != 15 {
		t.Fatal("returned totals mutated cache")
	}
	backwards, _ := GetTotalTrafficBetween(100, 105)
	if backwards["eth0"].Tx != 5 {
		t.Fatalf("backwards=%v", backwards)
	}
}

func TestTrafficRangeMatchesScanAfterFlushAndPurge(t *testing.T) {
	setupStatic(t)
	mu.Lock()
	store.Interfaces["eth0"] = []TrafficData{{Timestamp: 100, Tx: 5}, {Timestamp: 110, Tx: 7}}
	staticCache["eth0"] = []TrafficData{{Timestamp: 111, Tx: 3}}
	mu.Unlock()
	_, _ = GetTotalTrafficBetween(100, 111)
	mu.Lock()
	flushCacheLocked(120)
	mu.Unlock()
	got, _ := GetTotalTrafficBetween(100, 111)
	if !reflect.DeepEqual(got, map[string]TrafficData{"eth0": {Tx: 12}}) {
		t.Fatalf("flush timestamp semantics changed: %v", got)
	}
	mu.Lock()
	purgeExpiredLocked()
	mu.Unlock()
	got, _ = GetTotalTrafficBetween(100, 130)
	if len(got) != 0 {
		t.Fatalf("expired cache totals remain: %v", got)
	}
}

func TestCollectorReloadWaitsForOldGenerationAndStop(t *testing.T) {
	setupStatic(t)
	if err := StartOrContinue(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		mu.RLock()
		previous := generation
		mu.RUnlock()
		if err := SetNewConfig(NetStaticConfig{DetectInterval: 0.001, SaveInterval: 0.002}); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() { previous.workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("old collectors still alive after reload")
		}
	}
	mu.RLock()
	last := generation
	mu.RUnlock()
	done := make(chan error, 1)
	go func() { done <- Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop deadlocked waiting while holding data lock")
	}
	last.workers.Wait()
}
