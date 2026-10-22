package metric

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestRetentionInfoReportsDefinitionOverrideWithoutWrites(t *testing.T) {
	ctx := context.Background()
	policy := RollupPolicy{Tiers: []RollupTier{{time.Minute, 3 * time.Hour}, {5 * time.Minute, 12 * time.Hour}, {time.Hour, 7 * 24 * time.Hour}, {24 * time.Hour, 30 * 24 * time.Hour}}}
	s := newRollupStore(t, policy)
	for name, days := range map[string]int{"long": 90, "short": 1, "disabled": 0} {
		if err := s.UpsertMetric(ctx, Definition{Name: name, Type: TypeGauge, RetentionDays: days}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := s.ListMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.RetentionInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.RawRetentionSeconds != 600 || len(info.Metrics) != 3 {
		t.Fatalf("unexpected info: %+v", info)
	}
	for _, m := range info.Metrics {
		if !reflect.DeepEqual(m.ConfiguredTiers, retentionTiers(policy.Tiers, "store_policy")) {
			t.Fatalf("configured changed: %+v", m)
		}
		switch m.MetricName {
		case "long":
			last := m.EffectiveTiers[len(m.EffectiveTiers)-1]
			if last.RetentionSeconds != 90*86400 || last.Source != "metric_definition" {
				t.Fatalf("definition must win: %+v", m)
			}
		case "short":
			if len(m.EffectiveTiers) != 3 || m.EffectiveTiers[2].RetentionSeconds != 86400 {
				t.Fatalf("pruned effective ladder: %+v", m)
			}
		case "disabled":
			if m.Enabled || len(m.EffectiveTiers) != 0 {
				t.Fatalf("disabled metric: %+v", m)
			}
		}
	}
	after, err := s.ListMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("inspection changed definitions")
	}
	info.Metrics[0].ConfiguredTiers[0].RetentionSeconds = 1
	again, err := s.RetentionInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.Metrics[0].ConfiguredTiers[0].RetentionSeconds != 10800 {
		t.Fatal("inspection mutated store policy")
	}
}
