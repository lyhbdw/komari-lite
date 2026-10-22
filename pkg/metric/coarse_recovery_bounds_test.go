package metric

import (
	"context"
	"testing"
	"time"
)

func TestRecoveryOnlyReadsUnsealedWindow(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "bounded", RetentionDays: 120}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	policy := s.cfg.RollupPolicy.withMetricRetention(120 * 24 * time.Hour)
	for i := 1; i < len(policy.Tiers); i++ {
		parent := policy.Tiers[i].Interval
		start := now.Add(-parent - coarseRollupGrace).Truncate(parent)
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		buckets := make(map[rollupKey]*rollupBucket)
		// Corrupt old digests prove the historical rows are not even decoded.
		for _, at := range []time.Time{start.Add(-parent), start, now.Truncate(parent)} {
			b := newRollupBucket(30)
			b.tagsHash, b.tagsJSON = emptyLabelsHash, "{}"
			b.labelsHash, b.labelsJSON = emptyLabelsHash, "{}"
			b.addPoint(1, at.UnixMilli())
			b.addPoint(2, at.Add(time.Second).UnixMilli())
			buckets[rollupKey{entityID: "n1", tagsHash: emptyLabelsHash, labelsHash: emptyLabelsHash, bucket: at.UnixMilli()}] = b
		}
		if _, err = s.writeRollupBucketsTx(ctx, "bounded", policy.Tiers[i-1].Interval, buckets, tx); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.ExecContext(ctx, "UPDATE "+s.tables.rollups+" SET digest = X'00' WHERE bucket_milli < ?", start.UnixMilli()); err != nil {
			t.Fatal(err)
		}
		if err = s.recoverCoarseTier(ctx, "bounded", policy, i, now); err != nil {
			t.Fatalf("historical rows scanned for %s: %v", parent, err)
		}
		for key := range s.coarse {
			if key.interval == parent && key.bucket+parent.Milliseconds()+coarseRollupGrace.Milliseconds() <= now.UnixMilli() {
				t.Fatalf("recovered parent beyond sealing deadline: %#v", key)
			}
			if key.interval == parent && key.bucket < start.UnixMilli() {
				t.Fatalf("recovered sealed parent: %#v", key)
			}
		}
	}
}
