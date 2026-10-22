package metric

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in scale verification must only receive a disposable scratch copy.
func TestRecoveryProductionScaleCopy(t *testing.T) {
	path := os.Getenv("KOMARI_RECOVERY_SCALE_COPY")
	if path == "" {
		t.Skip("set KOMARI_RECOVERY_SCALE_COPY to a disposable scratch copy")
	}
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, "/root/.hermes/cache/scratch/") {
		t.Fatal("scale test refuses non-scratch databases")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var beforeCount, beforePoints int64
	var beforeSum float64
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*),SUM(count),SUM(sum) FROM metric_rollups").Scan(&beforeCount, &beforePoints, &beforeSum); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.closeDBs() // Verify startup only; do not persist scratch parents via Close.
	elapsed := time.Since(started)
	if elapsed >= coarseRecoveryTimeout {
		t.Fatalf("Open took %s, recovery was not completed within budget", elapsed)
	}
	// Compare each recovered child against the exact persisted source row. No
	// reconstructed parent may already exist in storage.
	children := 0
	for key, parent := range s.coarse {
		var childInterval time.Duration
		policy, err := s.metricRollupPolicy(ctx, key.metricName)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(policy.Tiers); i++ {
			if policy.Tiers[i].Interval == key.interval {
				childInterval = policy.Tiers[i-1].Interval
			}
		}
		var present int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM metric_rollups r JOIN metric_series s ON s.id=r.series_id JOIN metric_resolutions d ON d.id=r.resolution_id JOIN metric_label_sets l ON l.id=r.label_id WHERE s.metric_name=? AND s.entity_id=? AND s.tags_hash=? AND l.labels_hash=? AND d.resolution_milli=? AND r.bucket_milli=?`, key.metricName, key.entityID, key.tagsHash, key.labelsHash, key.interval.Milliseconds(), key.bucket).Scan(&present)
		if err != nil || present != 0 {
			t.Fatalf("recovered an existing parent: %v %d", err, present)
		}
		for ck, b := range parent.children {
			var count int64
			var sum float64
			err = db.QueryRowContext(ctx, `SELECT r.count,r.sum FROM metric_rollups r JOIN metric_series s ON s.id=r.series_id JOIN metric_resolutions d ON d.id=r.resolution_id JOIN metric_label_sets l ON l.id=r.label_id WHERE s.metric_name=? AND s.entity_id=? AND s.tags_hash=? AND l.labels_hash=? AND d.resolution_milli=? AND r.bucket_milli=?`, key.metricName, ck.entityID, ck.tagsHash, ck.labelsHash, childInterval.Milliseconds(), ck.bucket).Scan(&count, &sum)
			if err != nil || count != b.count || sum != b.sum {
				t.Fatalf("recovered child differs: %v %d %g vs %d %g", err, count, sum, b.count, b.sum)
			}
			children++
		}
	}
	var afterCount, afterPoints int64
	var afterSum float64
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*),SUM(count),SUM(sum) FROM metric_rollups").Scan(&afterCount, &afterPoints, &afterSum); err != nil {
		t.Fatal(err)
	}
	if beforeCount != afterCount || beforePoints != afterPoints || beforeSum != afterSum {
		t.Fatal("Open rewrote persisted summaries")
	}
	fmt.Printf("scale_open_elapsed=%s persisted_rows=%d persisted_points=%d recovered_parents=%d recovered_children=%d\n", elapsed, beforeCount, beforePoints, len(s.coarse), children)
	// The copy may contain only sealed parents; a separate real crash test
	// establishes nonempty recovery correctness in that case.
}
