package metric

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

// Delay only the startup recovery lookup; real SQLite ping/config/schema access
// still exercises Open. The delay ends at the actual recovery context deadline.
type recoveryBudgetDriver struct{ sqlite3.SQLiteDriver }
type recoveryBudgetConn struct{ driver.Conn }

func (d *recoveryBudgetDriver) Open(name string) (driver.Conn, error) {
	c, err := d.SQLiteDriver.Open(name)
	if err != nil {
		return nil, err
	}
	return recoveryBudgetConn{c}, nil
}
func (c recoveryBudgetConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(q, "SELECT EXISTS (SELECT 1 FROM sqlite_master") {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}

var recoveryBudgetDriverOnce sync.Once

func TestOpenRecoveryDeadlineDoesNotBlockReadiness(t *testing.T) {
	recoveryBudgetDriverOnce.Do(func() { sql.Register("metric-recovery-budget", &recoveryBudgetDriver{}) })
	for _, budget := range []time.Duration{50 * time.Millisecond, 0} {
		t.Run(budget.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "budget.db")
			initial, err := Open(context.Background(), recoveryTestConfig(path))
			if err != nil {
				t.Fatal(err)
			}
			if err = initial.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("metric-recovery-budget", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			var cancel context.CancelFunc
			if budget > 0 {
				ctx, cancel = context.WithTimeout(ctx, budget)
				defer cancel()
			}
			start := time.Now()
			s, err := Open(ctx, SQLite(path, WithDB(db), WithAutoMigrate(false)))
			if err != nil {
				t.Fatalf("recovery timeout must not fail Open: %v", err)
			}
			defer s.Close()
			max := coarseRecoveryTimeout + time.Second
			if budget > 0 {
				max = budget + time.Second
			}
			if elapsed := time.Since(start); elapsed > max {
				t.Fatalf("Open exceeded recovery budget: %s > %s", elapsed, max)
			}
			if err = s.db.PingContext(context.Background()); err != nil {
				t.Fatalf("store not ready after recovery timeout: %v", err)
			}
		})
	}
}

func TestOpenRecoveryErrorDiscardsPartialParents(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "corrupt.db")
	base := time.Now().UTC().Truncate(5 * time.Minute).Add(-5 * time.Minute)
	runRecoveryCrashWriter(t, path, base, "minute")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "UPDATE metric_rollups SET digest = X'00'"); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatalf("recovery error blocked Open: %v", err)
	}
	defer s.Close()
	if len(s.coarse) != 0 || len(s.hotBaseline) != 0 {
		t.Fatal("failed recovery retained partial state")
	}
	var count int64
	if err = s.db.QueryRowContext(ctx, "SELECT SUM(count) FROM metric_rollups").Scan(&count); err != nil || count != 2 {
		t.Fatalf("committed summaries changed: %d, %v", count, err)
	}
}

func TestRecoveryChildRowLimitDiscardsPartialState(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "limit", RetentionDays: 120}); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Minute)
	writeRollupPointsAt(t, ctx, s, base.Add(time.Minute), Point{MetricName: "limit", EntityID: "seed", Timestamp: base, Value: 1})
	// Each row is a valid separate entity in the same minute.
	_, err := s.db.ExecContext(ctx, `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x <= ?)
 INSERT INTO metric_series(metric_name,entity_id,tags_hash,tags) SELECT 'limit','n'||x,tags_hash,tags FROM n CROSS JOIN metric_series WHERE entity_id='seed'`, coarseRecoveryRowLimit)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO metric_rollups SELECT s.id,r.resolution_id,r.label_id,r.bucket_milli,r.count,r.sum,r.sum_sq,r.min_val,r.max_val,r.first_val,r.first_ts_milli,r.last_val,r.last_ts_milli,r.digest,r.created_at_milli FROM metric_series s CROSS JOIN metric_rollups r WHERE s.entity_id!='seed'`)
	if err != nil {
		t.Fatal(err)
	}
	s.coarse = make(map[coarseRollupKey]*coarseRollup)
	err = s.recoverCoarseRollups(ctx, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "row limit") {
		t.Fatalf("row cap not enforced: %v", err)
	}
	s.recoverCoarseRollupsBestEffort(ctx)
	if len(s.coarse) != 0 || len(s.hotBaseline) != 0 {
		t.Fatal("row-limited recovery left partial parents")
	}
}
