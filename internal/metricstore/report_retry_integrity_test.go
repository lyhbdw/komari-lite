package metricstore

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/pkg/metric"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func TestReportRetryAfterSQLiteFailureKeepsTrafficAndSampleCount(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "retry.db")
	s, err := openStore(ctx, &MetricStoreConfig{DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storeMu.Lock()
	old := store
	store = s
	storeMu.Unlock()
	defer func() { storeMu.Lock(); store = old; storeMu.Unlock(); clearReportTrafficStates() }()
	clearReportTrafficStates()
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute + time.Second)
	report := v2.Report{UUID: "sql-failure", UpdatedAt: base, CPU: v2.CPUReport{Usage: 10}, Network: v2.NetworkReport{TotalUp: 100, TotalDown: 200}}
	if _, err := writeReportBatch(ctx, []v2.Report{report}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_report_insert BEFORE INSERT ON metric_rollups BEGIN SELECT RAISE(FAIL,'isolated write failure'); END`); err != nil {
		t.Fatal(err)
	}
	report.UpdatedAt = base.Add(time.Second)
	report.CPU.Usage = 30
	report.Network.TotalUp = 150
	report.Network.TotalDown = 260
	pending := []v2.Report{report}
	if err := writePendingReports(ctx, &pending); err == nil {
		t.Fatal("fault injection did not fail")
	}
	if len(pending) != 1 {
		t.Fatal("failed sample was discarded")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_report_insert`); err != nil {
		t.Fatal(err)
	}
	if err := writePendingReports(ctx, &pending); err != nil {
		t.Fatal(err)
	}
	wantValues := map[string][]float64{MetricCPU: {10, 30}, MetricTrafficUp: {0, 50}, MetricTrafficDown: {0, 60}}
	for name, want := range wantValues {
		assertMetricValues(t, s, name, report.UUID, base.Add(-time.Second), base.Add(time.Minute), want)
	}
	for name, want := range map[string]float64{MetricCPU: 20, MetricTrafficUp: 50, MetricTrafficDown: 60} {
		agg := metric.AggAvg
		if name != MetricCPU {
			agg = metric.AggSum
		}
		points, err := s.Series(ctx, metric.AggregateQuery{Query: metric.Query{MetricName: name, EntityID: report.UUID, Start: base.Add(-time.Second), End: base.Add(time.Minute)}, Aggregation: agg, Interval: time.Minute}, time.Now().UTC())
		if err != nil || len(points) != 1 || points[0].Value != want || points[0].Count != 2 {
			t.Fatalf("%s retry corrupted aggregate: %s, err=%v", name, fmt.Sprint(points), err)
		}
	}
}
