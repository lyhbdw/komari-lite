package metric

import (
	"context"
	"testing"
	"time"
)

func TestReplaceRollupPointsIsIdempotentAndSkipsRawWindow(t *testing.T) {
	ctx := context.Background()
	policy := RollupPolicy{
		RawRetention: time.Minute,
		Tiers: []RollupTier{
			{Interval: time.Minute, Retention: 10 * time.Hour},
			{Interval: time.Hour, Retention: 30 * 24 * time.Hour},
			{Interval: 24 * time.Hour, Retention: 365 * 24 * time.Hour},
		},
		Compression: 30,
	}
	store := newRollupStore(t, policy)
	if err := store.UpsertMetric(ctx, Definition{Name: "legacy.p95", RetentionDays: 365}); err != nil {
		t.Fatalf("create metric: %v", err)
	}
	base := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	points := []Point{
		{MetricName: "legacy.p95", EntityID: "node-a", Timestamp: base, Value: 95},
		{MetricName: "legacy.p95", EntityID: "node-a", Timestamp: base.Add(time.Hour), Value: 195},
	}
	for i := 0; i < 2; i++ {
		if err := store.ReplaceRollupPoints(ctx, time.Hour, points); err != nil {
			t.Fatalf("replace hourly points pass %d: %v", i+1, err)
		}
	}
	raw, err := store.Query(ctx, Query{MetricName: "legacy.p95", EntityID: "node-a", Start: base.Add(-time.Minute), End: base.Add(2 * time.Hour)})
	if err != nil {
		t.Fatalf("query raw window: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("pre-aggregated import entered raw window: %#v", raw)
	}

	// idempotency: verify the imported hourly points are readable and stable
	hourly, err := store.SeriesBatch(ctx, BatchSeriesQuery{
		Specs:      []BatchSeriesSpec{{MetricName: "legacy.p95", Aggregations: []Aggregation{AggLast}, Interval: time.Hour}},
		EntityIDs:  []string{"node-a"},
		Start:      base.Add(-time.Minute),
		End:        base.Add(2 * time.Hour),
		Order:      OrderAsc,
	}, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("read hourly import: %v", err)
	}
	got := hourly.Values["legacy.p95"][AggLast]
	if len(got) != 2 || got[0].Count != 1 || got[0].Value != 95 || got[1].Value != 195 {
		t.Fatalf("idempotent hourly import = %#v", got)
	}
}
