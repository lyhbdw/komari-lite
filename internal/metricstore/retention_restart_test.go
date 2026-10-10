package metricstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/pkg/metric"
)

func TestRetentionProfileIsExplicitValidatedAndRestartSafe(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restart.db")
	legacy := &MetricStoreConfig{DSN: path}
	s, err := openStore(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Hour).Add(-45 * 24 * time.Hour)
	if err := s.ReplaceRollupPoints(ctx, time.Hour, []metric.Point{{MetricName: MetricCPU, EntityID: "historical", Timestamp: base, Value: 17}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	lightweight := &MetricStoreConfig{DSN: path, RetentionProfile: RetentionProfileLightweight}
	s, err = openStore(ctx, lightweight)
	if err != nil {
		t.Fatal(err)
	}
	cpu, err := s.GetMetric(ctx, MetricCPU)
	if err != nil || cpu.RetentionDays != 30 {
		t.Fatalf("explicit restart: %#v %v", cpu, err)
	}
	// Opening a shorter policy changes definitions but must not delete historical
	// rollups. Cleanup remains an explicit scheduled/admin operation.
	result, err := s.Series(ctx, metric.AggregateQuery{Query: metric.Query{MetricName: MetricCPU, EntityID: "historical", Start: base, End: base.Add(time.Hour)}, Aggregation: metric.AggSum, Interval: time.Hour}, base.Add(time.Hour))
	if err != nil || len(result) != 1 || result[0].Value != 17 {
		t.Fatalf("open deleted historical data: %#v %v", result, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = openStore(ctx, lightweight)
	if err != nil {
		t.Fatal(err)
	}
	cpu, err = s.GetMetric(ctx, MetricCPU)
	if err != nil || cpu.RetentionDays != 30 {
		t.Fatalf("lightweight restart changed policy: %#v %v", cpu, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = openStore(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cpu, err = s.GetMetric(ctx, MetricCPU)
	if err != nil || cpu.RetentionDays != 90 {
		t.Fatalf("omitted profile must retain legacy contract: %#v %v", cpu, err)
	}
	for _, invalid := range []string{"light", "Lightweight", " lightweight ", "default"} {
		cfg := &MetricStoreConfig{RetentionProfile: invalid}
		if _, err := buildMetricConfig(cfg, false); err == nil {
			t.Fatalf("unknown profile %q accepted", invalid)
		}
	}
	if _, err := builtinRetentionFromConfig(nil, 90); err == nil {
		t.Fatal("nil config accepted")
	}
	for _, profile := range []string{"", RetentionProfileLegacy} {
		cfg := &MetricStoreConfig{RetentionProfile: profile, RollupMinuteRetentionMinutes: 120, RollupFiveMinuteRetentionMinutes: 1440, RollupHourRetentionHours: 240, RollupDayRetentionDays: 365}
		policy, err := rollupPolicyFromConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if policy.Tiers[0].Retention != 2*time.Hour || policy.Tiers[3].Retention != 365*24*time.Hour {
			t.Fatalf("legacy overrides lost: %#v", policy)
		}
		days, err := builtinRetentionFromConfig(cfg, 10)
		if err != nil || days != 90 {
			t.Fatalf("migration default chose new profile: %d %v", days, err)
		}
	}
}
