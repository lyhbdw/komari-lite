package metric

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSnapshotToCreatesConsistentSQLiteCopy(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "metrics.db")
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	store, err := Open(ctx, SQLite(source))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer store.Close()
	if err := store.CreateMetric(ctx, Definition{Name: "cpu", Type: TypeGauge}); err != nil {
		t.Fatalf("create metric: %v", err)
	}

	if err := store.SnapshotTo(ctx, snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	db, err := sql.Open("sqlite3", snapshot)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM metric_definitions WHERE name = 'cpu'").Scan(&count); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	if count != 1 {
		t.Fatalf("snapshot metric count = %d, want 1", count)
	}
}
