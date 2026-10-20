//go:build linux

package monitoring

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/agent/internal/netsample"
	"github.com/lyhbdw/komari-lite/agent/monitoring/netstatic"
	"github.com/shirou/gopsutil/v4/net"
)

func topologyFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "sys/class/net", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string]string{"uevent": "", "type": "1\n"} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := flags.HostProc
	flags.HostProc = filepath.Join(root, "proc")
	t.Cleanup(func() { flags.HostProc = old })
	return dir
}

func TestTopologyCacheMonthlyTotalsRefreshBeforeFiltering(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	if err := netstatic.Stop(); err != nil {
		t.Fatal(err)
	}
	oldPath := netstatic.SaveFilePath
	oldMonth, oldInclude, oldExclude := flags.MonthRotate, flags.IncludeNics, flags.ExcludeNics
	netstatic.SaveFilePath = filepath.Join(t.TempDir(), "net.json")
	t.Cleanup(func() {
		if err := netstatic.Stop(); err != nil {
			t.Error(err)
		}
		netstatic.SaveFilePath = oldPath
		flags.MonthRotate, flags.IncludeNics, flags.ExcludeNics = oldMonth, oldInclude, oldExclude
	})
	data := fmt.Sprintf(`{"interfaces":{"uplink0":[{"timestamp":%d,"tx":123,"rx":456}]}}`, time.Now().Unix())
	if err := os.WriteFile(netstatic.SaveFilePath, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	counters, err := netsample.Counters()
	if err != nil {
		t.Fatal(err)
	}
	refreshNetworkTopology(counters, nil, nil)
	if isCountedElsewhere("uplink0") {
		t.Fatal("initial device excluded")
	}
	if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	flags.MonthRotate, flags.IncludeNics, flags.ExcludeNics = 1, "", "unused*"
	up, down, _, _, err := NetworkSpeed()
	if err != nil {
		t.Fatal(err)
	}
	if up != 0 || down != 0 {
		t.Fatalf("monthly filter used stale classification before config invalidation: %d/%d", up, down)
	}
}

func TestTopologyCacheHostProcChange(t *testing.T) {
	first := topologyFixture(t, "uplink0")
	firstProc := flags.HostProc
	second := topologyFixture(t, "uplink0")
	secondProc := flags.HostProc
	if err := os.Mkdir(filepath.Join(second, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	var c topologyCache
	if c.classify("uplink0", firstProc) {
		t.Fatal("first host unexpectedly excluded")
	}
	if !c.classify("uplink0", secondProc) {
		t.Fatal("HostProc change did not invalidate")
	}
	if err := os.Mkdir(filepath.Join(first, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	if !c.classify("uplink0", firstProc) {
		t.Fatal("returning HostProc reused historical cache")
	}
}

func TestTopologyCacheFallbackWithoutHostSys(t *testing.T) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil || len(entries) == 0 {
		t.Fatal("host sysfs required for fallback fixture")
	}
	name := entries[0].Name()
	want, _ := inspectNetworkTopology(name, "")
	got, _ := inspectNetworkTopology(name, filepath.Join(t.TempDir(), "proc"))
	if got != want {
		t.Fatalf("missing HostProc sysfs fallback changed: got=%v want=%v", got, want)
	}
}

func TestTopologyCacheReadErrorsRetried(t *testing.T) {
	for _, file := range []string{"uevent", "type"} {
		t.Run(file, func(t *testing.T) {
			dir := topologyFixture(t, "uplink0")
			path := filepath.Join(dir, file)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatal(err)
			}
			var c topologyCache
			c.classify("uplink0", flags.HostProc)
			if len(c.entries) != 0 {
				t.Fatalf("%s read error cached", file)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("1\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
				t.Fatal(err)
			}
			if !c.classify("uplink0", flags.HostProc) {
				t.Fatal("read error prevented retry")
			}
		})
	}
}

func TestTopologyCacheSameSampleOrderPreservesHit(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	counters := []net.IOCountersStat{{Name: "uplink0"}, {Name: "lo"}}
	exclude := map[string]struct{}{"foo*": {}, "bar*": {}}
	networkTotalsFromCounters(counters, nil, exclude)
	if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	counters[0], counters[1] = counters[1], counters[0]
	_, _, names, _ := networkTotalsFromCounters(counters, nil, map[string]struct{}{"bar*": {}, "foo*": {}})
	if len(names) != 1 || names[0] != "uplink0" {
		t.Fatal("unchanged set/order variation caused cache I/O")
	}
}

func TestTopologyCacheFilterPriority(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	if err := os.Mkdir(filepath.Join(dir, "lower_eth0"), 0755); err != nil {
		t.Fatal(err)
	}
	if shouldInclude("uplink0", nil, nil) {
		t.Fatal("topology not applied by default")
	}
	if !shouldInclude("uplink0", map[string]struct{}{"up*": {}}, map[string]struct{}{"up*": {}}) {
		t.Fatal("explicit include must beat exclude and topology")
	}
	if shouldInclude("uplink0", nil, map[string]struct{}{"up*": {}}) {
		t.Fatal("explicit exclude ignored")
	}
	if shouldInclude("lo", map[string]struct{}{"*": {}}, nil) {
		t.Fatal("loopback must beat explicit include")
	}
}

func TestTopologyCacheConcurrent(t *testing.T) {
	var calls atomic.Int64
	c := topologyCache{now: func() time.Time { return time.Unix(100, 0) }, inspect: func(string, string) (bool, bool) {
		calls.Add(1)
		return true, true
	}}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if !c.classify("uplink0", "root") {
					t.Error("concurrent classification changed")
				}
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent cold misses repeated I/O: %d", calls.Load())
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				c.observeSample("root", fmt.Sprintf("%d", j%2))
				if !c.classify(fmt.Sprintf("nic%d", i), "root") {
					t.Error("classification lost during invalidation")
				}
			}
		}(i)
	}
	wg.Wait()
	if len(c.entries) > 256 {
		t.Fatal("concurrent invalidation exceeded cache bound")
	}
}

func TestTopologyCacheClassificationRebaselinesSpeed(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	networkSpeedSample.Lock()
	oldEpoch, oldUp, oldDown, oldAt := networkSpeedSample.epoch, networkSpeedSample.totalUp, networkSpeedSample.totalDown, networkSpeedSample.sampledAt
	networkSpeedSample.sampledAt = time.Time{}
	networkSpeedSample.Unlock()
	t.Cleanup(func() {
		networkSpeedSample.Lock()
		networkSpeedSample.epoch, networkSpeedSample.totalUp, networkSpeedSample.totalDown, networkSpeedSample.sampledAt = oldEpoch, oldUp, oldDown, oldAt
		networkSpeedSample.Unlock()
	})
	var clock atomic.Int64
	clock.Store(100)
	networkTopologyCache.mu.Lock()
	networkTopologyCache.entries = nil
	networkTopologyCache.now = func() time.Time { return time.Unix(clock.Load(), 0) }
	networkTopologyCache.mu.Unlock()
	t.Cleanup(func() {
		networkTopologyCache.mu.Lock()
		networkTopologyCache.now = nil
		networkTopologyCache.entries = nil
		networkTopologyCache.mu.Unlock()
	})
	now := time.Unix(100, 0)
	counters := []net.IOCountersStat{{Name: "uplink0", BytesSent: 1000, BytesRecv: 2000}}
	sample := func(at time.Time) (uint64, uint64) {
		up, down, names, err := networkTotalsFromCounters(counters, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		return updateNetworkSpeedSample(up, down, calcNicEpoch("topology-test", names), at)
	}
	sample(now)
	counters[0].BytesSent += 100
	counters[0].BytesRecv += 200
	if up, down := sample(now.Add(time.Second)); up != 100 || down != 200 {
		t.Fatalf("ordinary delta changed: %d/%d", up, down)
	}
	if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	clock.Store(105)
	if up, down := sample(now.Add(5 * time.Second)); up != 0 || down != 0 {
		t.Fatalf("exclusion did not rebaseline: %d/%d", up, down)
	}
	if err := os.Remove(filepath.Join(dir, "master")); err != nil {
		t.Fatal(err)
	}
	clock.Store(110)
	counters[0].BytesSent = 1000000000
	counters[0].BytesRecv = 2000000000
	if up, down := sample(now.Add(10 * time.Second)); up != 0 || down != 0 {
		t.Fatalf("re-inclusion caused traffic spike: %d/%d", up, down)
	}
	counters[0].BytesSent += 100
	counters[0].BytesRecv += 200
	if up, down := sample(now.Add(11 * time.Second)); up != 100 || down != 200 {
		t.Fatalf("post-baseline delta incorrect: %d/%d", up, down)
	}
}

func TestTopologyCacheRetriesHostSysErrors(t *testing.T) {
	root := t.TempDir()
	sysDir := filepath.Join(root, "sys/class/net")
	if err := os.MkdirAll(filepath.Dir(sysDir), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("net", sysDir); err != nil {
		t.Fatal(err)
	}
	hostProc := filepath.Join(root, "proc")
	var c topologyCache
	// Use an interface that really exists so the original host-sysfs fallback is exercised.
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil || len(entries) == 0 {
		t.Fatal("host sysfs required for fallback fixture")
	}
	name := entries[0].Name()
	expected, _ := inspectNetworkTopology(name, "")
	if got := c.classify(name, hostProc); got != expected {
		t.Fatal("HostProc fallback classification changed")
	}
	if len(c.entries) != 0 {
		t.Fatal("HostProc sysfs error was cached")
	}
}

func TestTopologyCacheRetriesStatErrors(t *testing.T) {
	for _, entry := range []string{"device", "brport", "master"} {
		t.Run(entry, func(t *testing.T) {
			dir := topologyFixture(t, "uplink0")
			path := filepath.Join(dir, entry)
			if err := os.Symlink(entry, path); err != nil {
				t.Fatal(err)
			}
			var c topologyCache
			if c.classify("uplink0", flags.HostProc) {
				t.Fatal("broken link unexpectedly excluded device")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
				t.Fatal(err)
			}
			if !c.classify("uplink0", flags.HostProc) {
				t.Fatalf("%s stat error was cached", entry)
			}
		})
	}
}

func TestTopologyCacheSampleInvalidation(t *testing.T) {
	for _, change := range []string{"added", "removed", "exclude", "include"} {
		t.Run(change, func(t *testing.T) {
			dir := topologyFixture(t, "uplink0")
			counters := []net.IOCountersStat{{Name: "uplink0", BytesSent: 10, BytesRecv: 20}}
			if change == "removed" {
				counters = append(counters, net.IOCountersStat{Name: "lo"})
			}
			_, _, counted, _ := networkTotalsFromCounters(counters, nil, nil)
			if len(counted) != 1 {
				t.Fatal("initial device not counted")
			}
			if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
				t.Fatal(err)
			}
			var exclude map[string]struct{}
			switch change {
			case "added":
				counters = append(counters, net.IOCountersStat{Name: "lo"})
			case "removed":
				counters = counters[:1]
			case "exclude":
				exclude = map[string]struct{}{"other*": {}}
			case "include":
				networkTotalsFromCounters(counters, map[string]struct{}{"up*": {}}, nil)
			}
			up, down, counted, _ := networkTotalsFromCounters(counters, nil, exclude)
			if up != 0 || down != 0 || len(counted) != 0 {
				t.Fatalf("%s change did not invalidate: up=%d down=%d nics=%v", change, up, down, counted)
			}
		})
	}
}

func TestTopologyCacheRetriesMissingDevice(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	var c topologyCache
	if c.classify("uplink0", flags.HostProc) {
		t.Fatal("missing device must fail open")
	}
	if err := os.MkdirAll(filepath.Join(dir, "master"), 0755); err != nil {
		t.Fatal(err)
	}
	if !c.classify("uplink0", flags.HostProc) {
		t.Fatal("missing device was cached instead of retried")
	}
}

func TestTopologyCacheBounded(t *testing.T) {
	c := topologyCache{inspect: func(string, string) (bool, bool) { return false, true }}
	for i := 0; i < 1024; i++ {
		c.classify(fmt.Sprintf("nic%d", i), "")
	}
	if len(c.entries) > 256 {
		t.Fatalf("cache exceeded 256 entries: %d", len(c.entries))
	}
}

func TestTopologyCacheTTLRefresh(t *testing.T) {
	for _, change := range []string{"recreate", "bridge", "lower"} {
		t.Run(change, func(t *testing.T) {
			dir := topologyFixture(t, "uplink0")
			now := time.Unix(100, 0)
			calls := 0
			c := topologyCache{now: func() time.Time { return now }, inspect: func(name, root string) (bool, bool) {
				calls++
				return inspectNetworkTopology(name, root)
			}}
			if c.classify("uplink0", flags.HostProc) {
				t.Fatal("plain device excluded")
			}
			switch change {
			case "recreate":
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte("DEVTYPE=vlan\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "bridge":
				if err := os.Mkdir(filepath.Join(dir, "master"), 0755); err != nil {
					t.Fatal(err)
				}
			case "lower":
				if err := os.Mkdir(filepath.Join(dir, "lower_eth0"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			now = now.Add(5*time.Second - time.Nanosecond)
			if c.classify("uplink0", flags.HostProc) || calls != 1 {
				t.Fatal("cache expired before TTL or performed I/O on hit")
			}
			now = now.Add(time.Nanosecond)
			if !c.classify("uplink0", flags.HostProc) || calls != 2 {
				t.Fatalf("TTL failed to refresh %s classification: calls=%d", change, calls)
			}
		})
	}
}

func TestTopologyCacheHitSkipsSysfs(t *testing.T) {
	dir := topologyFixture(t, "uplink0")
	if err := os.Mkdir(filepath.Join(dir, "lower_eth0"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isCountedElsewhere("uplink0") {
		t.Fatal("upper device must be excluded")
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if !isCountedElsewhere("uplink0") {
		t.Fatal("cache hit queried removed sysfs device instead of reusing classification")
	}
}
