package metricstore

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestExplicitLightweightProfileControlsTiersAndDefinitions(t *testing.T) {
	var cfg MetricStoreConfig
	// Simulate persisted, auto-filled legacy scalar defaults. The explicitly
	// selected profile must win rather than silently remaining on the old tiers.
	if err := json.Unmarshal([]byte(`{"metric_retention_profile":"lightweight","metric_rollup_minute_retention_minutes":600,"metric_rollup_five_minute_retention_minutes":3000,"metric_rollup_hour_retention_hours":600,"metric_rollup_day_retention_days":730}`), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.DSN = filepath.Join(t.TempDir(), "metrics.db")
	policy, err := rollupPolicyFromConfig(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{3 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}
	for i, retention := range want {
		if policy.Tiers[i].Retention != retention {
			t.Fatalf("tier %d retention=%s, want %s for explicit lightweight profile", i, policy.Tiers[i].Retention, retention)
		}
	}
	s, err := openStore(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defs, err := s.ListMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 21 {
		t.Fatalf("definitions=%d, want 21", len(defs))
	}
	for _, def := range defs {
		if def.RetentionDays != 30 {
			t.Fatalf("%s retention=%d, want explicit 30", def.Name, def.RetentionDays)
		}
	}
}

func TestMigrationEnsurePreservesExplicitRetention(t *testing.T) {
	ctx := context.Background()
	var cfg MetricStoreConfig
	if err := json.Unmarshal([]byte(`{"metric_retention_profile":"lightweight"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.DSN = ":memory:"
	s, err := openStore(ctx, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureBuiltinMetricDefinitions(ctx, s); err != nil {
		t.Fatal(err)
	}
	defs, err := s.ListMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range defs {
		if def.RetentionDays != 30 {
			t.Fatalf("migration silently changed %s from 30 to %d days", def.Name, def.RetentionDays)
		}
	}
}
