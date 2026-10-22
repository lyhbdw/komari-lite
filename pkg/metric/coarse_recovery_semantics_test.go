package metric

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCrashRecoveryPreservesTagsLabelsAndNewReplacement(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "tagged.db")
	base := recoveryBase()
	runRecoveryCrashWriter(t, path, base, "tagged")
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// The new post-restart sample changes labels without moving the committed
	// prefix to the new label set or retaining the stale post-restart child.
	p := Point{MetricName: "crash-recovery", EntityID: "n2", Timestamp: base.Add(30 * time.Second), Value: 40, Tags: map[string]string{"task": "a"}, Labels: map[string]string{"site": "east"}}
	writeRollupPointsAt(t, ctx, s, base.Add(6*time.Minute), p)
	p.Value = 50
	p.Labels = map[string]string{"site": "north"}
	writeRollupPointsAt(t, ctx, s, base.Add(6*time.Minute), p)
	if _, err = s.FlushCoarse(ctx, base.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, interval := range []time.Duration{time.Minute, 5 * time.Minute, time.Hour, 24 * time.Hour} {
		rows, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", interval)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]float64{}
		for _, r := range rows {
			if r.entityID != "n2" {
				continue
			}
			tags, err := decodeMapString(r.bucketData.tagsJSON)
			if err != nil {
				t.Fatal(err)
			}
			labels, err := decodeMapString(r.bucketData.labelsJSON)
			if err != nil {
				t.Fatal(err)
			}
			seen[tags["task"]+"/"+labels["site"]] += r.bucketData.sum
		}
		if len(seen) != 3 || seen["a/east"] != 7 || seen["a/north"] != 50 || seen["b/west"] != 9 {
			t.Fatalf("%s label/tag contributions = %#v", interval, seen)
		}
	}
	if len(s.hotBaseline) == 0 {
		t.Fatal("restart did not preserve committed prefix")
	}
	if _, err = s.Flush(ctx, base.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(s.hotBaseline) != 0 {
		t.Fatal("inactive restart prefixes did not expire")
	}
}

func TestRecoveryDoesNotRestoreExpiredOrDisabledTiers(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "retention.db")
	base := recoveryBase()
	runRecoveryCrashWriter(t, path, base, "minute")
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetMetricRetention(ctx, "crash-recovery", 0); err != nil {
		t.Fatal(err)
	}
	s.closeDBs()
	s, err = Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if len(s.coarse) != 0 || len(s.hotBaseline) != 0 {
		t.Fatal("disabled metric resurrected")
	}
	if _, err = s.FlushCoarse(ctx, base.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, tier := range s.cfg.RollupPolicy.Tiers {
		rows, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", tier.Interval)
		if err != nil || len(rows) != 0 {
			t.Fatalf("disabled %s rows: %d, %v", tier.Interval, len(rows), err)
		}
	}
}

func TestRecoverySkipsAlreadyPersistedParent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sealed.db")
	base := recoveryBase()
	runRecoveryCrashWriter(t, path, base, "five-minute")
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Mutate only the disposable fine summary; the committed parent must not be
	// rebuilt from a partial or later-corrected finer tier.
	if _, err = s.db.ExecContext(ctx, "UPDATE metric_rollups SET sum=999 WHERE resolution_id IN (SELECT id FROM metric_resolutions WHERE resolution_milli=60000)"); err != nil {
		t.Fatal(err)
	}
	s.coarse = make(map[coarseRollupKey]*coarseRollup)
	if err = s.recoverCoarseRollups(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	for key := range s.coarse {
		if key.interval == 5*time.Minute {
			t.Fatal("persisted parent rebuilt")
		}
	}
	if _, err = s.FlushCoarse(ctx, base.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, interval := range []time.Duration{5 * time.Minute, time.Hour, 24 * time.Hour} {
		rows, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", interval)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].bucketData.sum != 30 || rows[0].bucketData.count != 2 {
			t.Fatalf("sealed %s overwritten: %#v", interval, rows)
		}
	}
}
