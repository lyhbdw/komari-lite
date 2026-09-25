package agent

import (
	"testing"
	"time"

	v2 "github.com/Tumb1er1376/komari-monitor-lite/protocol/v2"
)

func TestGetLatestReportDeepCopiesGPU(t *testing.T) {
	mu.Lock()
	previousLatest := latestReport
	previousRecent := recentReports
	latestReport = make(map[string]*v2.Report)
	recentReports = make(map[string][]v2.Report)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		latestReport = previousLatest
		recentReports = previousRecent
		mu.Unlock()
	})

	RecordReport(v2.Report{
		UUID:      "gpu-node",
		UpdatedAt: time.Now().UTC(),
		GPU: &v2.GPUDetailReport{
			Count:        1,
			AverageUsage: 50,
			DetailedInfo: []v2.GPUDeviceInfo{{Name: "gpu0", Utilization: 50}},
		},
	})

	latest := GetLatestReport()
	report := latest["gpu-node"]
	if report == nil || report.GPU == nil {
		t.Fatal("expected GPU report to be present")
	}
	if len(report.GPU.DetailedInfo) != 1 || report.GPU.DetailedInfo[0].Name != "gpu0" {
		t.Fatalf("unexpected GPU detail: %#v", report.GPU.DetailedInfo)
	}

	// Mutate the returned copy; the cached report must stay untouched.
	report.GPU.DetailedInfo[0].Name = "tampered"
	report.GPU.Count = 99

	fresh := GetLatestReport()["gpu-node"]
	if fresh == nil || fresh.GPU == nil {
		t.Fatal("expected GPU report to still be present")
	}
	if fresh.GPU.DetailedInfo[0].Name != "gpu0" {
		t.Fatalf("GPU detail leaked through shared slice: %#v", fresh.GPU.DetailedInfo)
	}
	if fresh.GPU.Count != 1 {
		t.Fatalf("GPU struct leaked through shared pointer: %#v", fresh.GPU)
	}
}
