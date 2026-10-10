package metric

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Use only existing normalized indexes: the rollup UNIQUE index is series-first.
func TestRollupReadNarrowUsesExistingSeriesIndex(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite(":memory:", WithMaxOpenConns(1)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, size := range []int{1, 15, 16, 17, 64} {
		ids := make([]int64, size)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		plan := rollupReadPlan{SeriesIDs: ids, ResolutionID: 1, StartMilli: 1, EndMilli: 1000, Fields: rollupReadSum | rollupReadLast}
		sql := s.dialect.renderRollupRead(s.tables, s.cfg.TablePrefix+"rollups_resolution_bucket_idx", plan)
		rows, err := s.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+sql.Query, sql.Args...)
		if err != nil {
			t.Fatal(err)
		}
		var details []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			details = append(details, detail)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		text := strings.Join(details, "; ")
		t.Logf("series=%d: %s", size, text)
		if size <= 16 {
			if !strings.Contains(text, "series_id=? AND resolution_id=?") {
				t.Errorf("narrow series=%d is not using existing series-first index: %s", size, text)
			}
		} else if !strings.Contains(text, "USING INDEX metric_rollups_resolution_bucket_idx") {
			t.Errorf("wide series=%d should retain time-index scan: %s", size, text)
		}
	}
}

// Direct SELECT benchmark isolates selectivity from result aggregation costs.
// Fixtures are synthetic in-memory SQLite, not production measurements.
func BenchmarkRollupReadSelectivity(b *testing.B) {
	ctx := context.Background()
	s, err := Open(ctx, SQLite(":memory:", WithMaxOpenConns(1)))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if err := s.UpsertMetric(ctx, Definition{Name: "bench", RetentionDays: 90}); err != nil {
		b.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metric_resolutions(id,resolution_milli) VALUES(1,60000); INSERT INTO metric_label_sets(id,labels_hash,labels) VALUES(1,'empty','{}')`); err != nil {
		b.Fatal(err)
	}
	seriesStmt, err := tx.PrepareContext(ctx, `INSERT INTO metric_series(id,metric_name,entity_id,tags_hash,tags) VALUES(?, 'bench', ?, 'empty', '{}')`)
	if err != nil {
		b.Fatal(err)
	}
	rowStmt, err := tx.PrepareContext(ctx, `INSERT INTO metric_rollups(series_id,resolution_id,label_id,bucket_milli,count,sum,sum_sq,min_val,max_val,first_val,first_ts_milli,last_val,last_ts_milli,created_at_milli) VALUES(?,1,1,?,1,1,1,1,1,1,?,1,?,0)`)
	if err != nil {
		b.Fatal(err)
	}
	for i := 1; i <= 512; i++ {
		if _, err := seriesStmt.ExecContext(ctx, i, fmt.Sprintf("node-%d", i)); err != nil {
			b.Fatal(err)
		}
		for minute := 0; minute < 120; minute++ {
			ts := int64(minute) * time.Minute.Milliseconds()
			if _, err := rowStmt.ExecContext(ctx, i, ts, ts, ts); err != nil {
				b.Fatal(err)
			}
		}
	}
	_ = seriesStmt.Close()
	_ = rowStmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "ANALYZE"); err != nil {
		b.Fatal(err)
	}
	for _, width := range []int{1, 15, 16, 17, 512} {
		ids := make([]int64, width)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		plan := rollupReadPlan{SeriesIDs: ids, ResolutionID: 1, StartMilli: 30 * time.Minute.Milliseconds(), EndMilli: 39 * time.Minute.Milliseconds(), Fields: rollupReadSum}
		selected := s.dialect.renderRollupRead(s.tables, "metric_rollups_resolution_bucket_idx", plan)
		forcedTime := selected
		// Keep the pre-change always-time path as a benchmark comparison.
		forcedTime.Query = strings.Replace(selected.Query, "INDEXED BY sqlite_autoindex_metric_rollups_1", "INDEXED BY metric_rollups_resolution_bucket_idx", 1)
		for _, variant := range []struct {
			name string
			sql  renderedSQL
		}{{"adaptive", selected}, {"always_time", forcedTime}} {
			b.Run(fmt.Sprintf("series_%d/%s", width, variant.name), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					rows, err := s.db.QueryContext(ctx, variant.sql.Query, variant.sql.Args...)
					if err != nil {
						b.Fatal(err)
					}
					var series, bucket, count int64
					var sum float64
					n := 0
					for rows.Next() {
						if err := rows.Scan(&series, &bucket, &count, &sum); err != nil {
							b.Fatal(err)
						}
						n++
					}
					if err := rows.Err(); err != nil {
						b.Fatal(err)
					}
					_ = rows.Close()
					if n != width*10 {
						b.Fatalf("rows=%d, want %d", n, width*10)
					}
				}
			})
		}
	}
}
