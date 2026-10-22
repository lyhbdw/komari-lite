package metric

import (
	"context"
	"testing"
	"time"
)

// MemoryStats must report the real in-memory shape of the store: every
// process-local structure that survives on the heap between batches is
// counted, so an operator can tell which tier is actually holding memory.
func TestMemoryStatsReportsInMemoryShapeAndDigestFootprint(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite(":memory:", WithRollupPolicy(RollupPolicy{
		Tiers:       []RollupTier{{Interval: time.Minute, Retention: time.Hour}},
		Compression: 30,
	})))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	if err := s.CreateMetric(ctx, Definition{Name: "mem", RetentionDays: 1}); err != nil {
		t.Fatalf("create metric: %v", err)
	}
	base := time.Now().UTC().Truncate(time.Minute)
	if err := s.WriteBatch(ctx, []Point{
		{MetricName: "mem", EntityID: "n1", Timestamp: base.Add(10 * time.Second), Value: 10},
		{MetricName: "mem", EntityID: "n1", Timestamp: base.Add(20 * time.Second), Value: 20},
	}); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	stats := s.MemoryStats()
	if stats.RawSeries != 1 {
		t.Errorf("RawSeries = %d, want 1", stats.RawSeries)
	}
	if stats.RawSamples != 2 {
		t.Errorf("RawSamples = %d, want 2", stats.RawSamples)
	}
	if stats.HotBuckets != 1 {
		t.Errorf("HotBuckets = %d, want 1", stats.HotBuckets)
	}
	// Each unprocessed point is its own centroid until the digest merges.
	if stats.HotDigestCentroids != 2 {
		t.Errorf("HotDigestCentroids = %d, want 2", stats.HotDigestCentroids)
	}
	if stats.EstimatedRawBytes <= 0 {
		t.Errorf("EstimatedRawBytes = %d, want a positive estimate", stats.EstimatedRawBytes)
	}
	if stats.EstimatedHotBytes <= 0 {
		t.Errorf("EstimatedHotBytes = %d, want a positive estimate", stats.EstimatedHotBytes)
	}
}

// An empty store must report zeros rather than nil-map surprises, and the
// estimate must never be negative.
func TestMemoryStatsOnEmptyStoreIsZeroed(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite(":memory:"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	stats := s.MemoryStats()
	if stats != (MemoryStats{}) {
		t.Fatalf("empty store stats = %#v, want zero value", stats)
	}
}

// Pool stats must expose the configured single-writer pool so an operator can
// see the fixed write connection and the read pool without guessing.
func TestDBPoolStatsReportsConfiguredConnections(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite(":memory:", WithMaxOpenConns(1)))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	if err := s.CreateMetric(ctx, Definition{Name: "pool", RetentionDays: 1}); err != nil {
		t.Fatalf("create metric: %v", err)
	}
	if err := s.WriteBatch(ctx, []Point{
		{MetricName: "pool", EntityID: "n1", Timestamp: time.Now().UTC(), Value: 1},
	}); err != nil {
		t.Fatalf("write batch: %v", err)
	}
	stats := s.DBPoolStats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}
	if stats.OpenConnections < 1 {
		t.Errorf("OpenConnections = %d, want at least 1", stats.OpenConnections)
	}
	if stats.InUse < 0 || stats.Idle < 0 {
		t.Errorf("pool counters must not be negative: %#v", stats)
	}
}
