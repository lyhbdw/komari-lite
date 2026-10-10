package metric

import (
	"fmt"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStorageSizeAndReclaimSpace(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, SQLiteInDir(dir, WithSQLiteWALAutoCheckpoint(1_000_000)))
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if got := store.Driver(); got != DriverSQLite {
		t.Fatalf("Driver() = %q, want %q", got, DriverSQLite)
	}
	if got := store.MaintenanceAction(); got != MaintenanceVacuum {
		t.Fatalf("MaintenanceAction() = %q, want %q", got, MaintenanceVacuum)
	}

	if _, err := store.db.ExecContext(ctx, `CREATE TABLE reclaim_fixture (payload BLOB NOT NULL)`); err != nil {
		t.Fatalf("create reclaim fixture: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO reclaim_fixture (payload) VALUES (zeroblob(4194304))`); err != nil {
		t.Fatalf("populate reclaim fixture: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DROP TABLE reclaim_fixture`); err != nil {
		t.Fatalf("drop reclaim fixture: %v", err)
	}

	before, err := store.StorageSize(ctx)
	if err != nil {
		t.Fatalf("storage size before reclaim: %v", err)
	}
	path := filepath.Join(dir, "metrics.db")
	if want := sqliteFileSetSize(t, path); before != want {
		t.Fatalf("StorageSize() = %d, file sum = %d", before, want)
	}

	if err := store.ReclaimSpace(ctx); err != nil {
		t.Fatalf("reclaim sqlite space: %v", err)
	}
	after, err := store.StorageSize(ctx)
	if err != nil {
		t.Fatalf("storage size after reclaim: %v", err)
	}
	if want := sqliteFileSetSize(t, path); after != want {
		t.Fatalf("StorageSize() after reclaim = %d, file sum = %d", after, want)
	}
	if after >= before {
		t.Fatalf("reclaim did not reduce physical storage: before=%d after=%d", before, after)
	}
	if _, err := store.Query(ctx, Query{MetricName: "__health__", Start: time.Unix(0, 0), End: time.Now().UTC()}); err != nil {
		t.Fatalf("store unusable after reclaim: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if _, err := store.StorageSize(ctx); !errors.Is(err, ErrClosed) {
		t.Fatalf("StorageSize() after Close error = %v, want ErrClosed", err)
	}
	if err := store.ReclaimSpace(ctx); !errors.Is(err, ErrClosed) {
		t.Fatalf("ReclaimSpace() after Close error = %v, want ErrClosed", err)
	}
}

func TestCleanupOrphanedMetricData(t *testing.T) {
	ctx := context.Background()
	store := newMemStore(t)
	if err := store.CreateMetric(ctx, Definition{Name: "known", Type: TypeGauge, RetentionDays: 1}); err != nil {
		t.Fatalf("create known definition: %v", err)
	}
	// Simulate a pre-constraint store containing orphaned rows. New stores
	// reject this state through their database foreign keys.
	if _, err := store.db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys for legacy fixture: %v", err)
	}
	now := time.Now().UTC().UnixMilli()
	if _, err := store.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (metric_name, entity_id, tags_hash, tags) VALUES (?, ?, ?, ?)`, store.tables.series), "orphan", "node-1", "hash", "{}"); err != nil {
		t.Fatalf("seed orphan series: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (labels_hash, labels) VALUES (?, ?)`, store.tables.labels), emptyLabelsHash, "{}"); err != nil {
		t.Fatalf("seed orphan labels: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (resolution_milli) VALUES (?)`, store.tables.resolutions), time.Minute.Milliseconds()); err != nil {
		t.Fatalf("seed orphan resolution: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO %s (series_id, resolution_id, label_id, bucket_milli, count, sum, sum_sq, min_val, max_val, first_val, first_ts_milli, last_val, last_ts_milli, digest, created_at_milli)
		 SELECT s.id, r.id, l.id, ?, 1, 1, 1, 1, 1, 1, ?, 1, ?, NULL, ? FROM %s s, %s r, %s l WHERE s.metric_name = ?`,
		store.tables.rollups, store.tables.series, store.tables.resolutions, store.tables.labels,
	), now, now, now, now, "orphan"); err != nil {
		t.Fatalf("seed orphan rollup: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("restore foreign keys after legacy fixture: %v", err)
	}

	deleted, err := store.cleanupOrphanedMetricData(ctx)
	if err != nil {
		t.Fatalf("clean orphaned metric data: %v", err)
	}
	if deleted != 4 {
		t.Fatalf("deleted rows = %d, want 4", deleted)
	}
	for _, table := range []string{store.tables.series, store.tables.labels, store.tables.resolutions, store.tables.rollups} {
		var count int
		if err := store.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count); err != nil {
			t.Fatalf("count orphan rows in %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("orphan rows remain in %s: %d", table, count)
		}
	}
}

func TestMaintenanceMappings(t *testing.T) {
	if got := maintenanceActionFor(DriverSQLite); got != MaintenanceVacuum {
		t.Fatalf("maintenanceActionFor(sqlite) = %q, want %q", got, MaintenanceVacuum)
	}
	if sqliteVacuumSQL != "VACUUM" {
		t.Fatalf("sqliteVacuumSQL = %q, want VACUUM", sqliteVacuumSQL)
	}
}

func sqliteFileSetSize(t *testing.T, path string) int64 {
	t.Helper()
	var size int64
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("stat %q: %v", name, err)
		}
		size += info.Size()
	}
	return size
}
