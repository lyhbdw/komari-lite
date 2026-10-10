package agent

import (
	"testing"
	"time"

	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func TestReportsAfterReusesCapacityAndClearsExpiredPointers(t *testing.T) {
	cutoff := time.Now().UTC()
	reports := make([]v2.Report, 3, 8)
	reports[0] = v2.Report{UpdatedAt: cutoff.Add(-time.Second), GPU: &v2.GPUDetailReport{}}
	reports[1] = v2.Report{UpdatedAt: cutoff, UUID: "kept-first"}
	reports[2] = v2.Report{UpdatedAt: cutoff.Add(time.Second), UUID: "kept-last", GPU: &v2.GPUDetailReport{}}
	backing := &reports[0]
	kept := reportsAfter(reports, cutoff)
	if len(kept) != 2 || kept[0].UUID != "kept-first" || kept[1].UUID != "kept-last" {
		t.Fatalf("pruned reports = %#v", kept)
	}
	if &kept[0] != backing || cap(kept) != 8 {
		t.Fatal("pruning should reuse the internal backing slice")
	}
	if reports[2].GPU != nil || reports[2].UUID != "" {
		t.Fatal("pruned tail must be cleared so old report references can be collected")
	}
	if again := reportsAfter(kept, cutoff); &again[0] != backing {
		t.Fatal("unchanged window should not allocate a new slice")
	}
}

func TestReportsAfterAllocatesNothingWhenWindowIsUnchanged(t *testing.T) {
	cutoff := time.Now().UTC()
	reports := []v2.Report{{UpdatedAt: cutoff}, {UpdatedAt: cutoff.Add(time.Second)}}
	if got := testing.AllocsPerRun(100, func() { reports = reportsAfter(reports, cutoff) }); got != 0 {
		t.Fatalf("unchanged internal window allocated %g times, want 0", got)
	}
}

func TestRecentReportGPUReadDoesNotAliasRuntime(t *testing.T) {
	mu.Lock()
	oldLatest, oldRecent := latestReport, recentReports
	latestReport, recentReports = make(map[string]*v2.Report), make(map[string][]v2.Report)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		latestReport, recentReports = oldLatest, oldRecent
		mu.Unlock()
	})
	report := v2.Report{UUID: "node", GPU: &v2.GPUDetailReport{DetailedInfo: []v2.GPUDeviceInfo{{Name: "original"}}}}
	RecordReport(report)
	recent := GetRecentReports("node")
	recent[0].GPU.DetailedInfo[0].Name = "changed"
	if got := GetRecentReports("node")[0].GPU.DetailedInfo[0].Name; got != "original" {
		t.Fatalf("caller changed runtime GPU state: %s", got)
	}
}

func BenchmarkReportsAfterUnchanged(b *testing.B) {
	now := time.Now().UTC()
	reports := make([]v2.Report, 60, 128)
	for i := range reports {
		reports[i].UpdatedAt = now.Add(time.Duration(i) * time.Second)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reports = reportsAfter(reports, now)
	}
}
