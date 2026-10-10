package metricstore

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/pkg/metric"
	sqlite3 "github.com/mattn/go-sqlite3"
)

func TestLegacyRecordBatchBoundsReadWork(t *testing.T) {
	ctx := context.Background()
	var selects atomic.Int32
	dsn := fmt.Sprintf("file:legacy-batch-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db := sql.OpenDB(&reportSQLiteConnector{dsn: dsn, driver: &sqlite3.SQLiteDriver{ConnectHook: func(c *sqlite3.SQLiteConn) error {
		c.RegisterAuthorizer(func(op int, _, _, _ string) int {
			if op == sqlite3.SQLITE_SELECT {
				selects.Add(1)
			}
			return sqlite3.SQLITE_OK
		})
		return nil
	}}})
	s, err := metric.Open(ctx, metric.SQLite("", metric.WithDB(db), metric.WithMaxOpenConns(1), metric.WithRollupPolicy(mustDefaultPolicy(t))))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer s.Close()
	if err := createMetricDefinitions(ctx, s); err != nil {
		t.Fatal(err)
	}
	storeMu.Lock()
	old := store
	store = s
	storeMu.Unlock()
	defer func() { storeMu.Lock(); store = old; storeMu.Unlock() }()
	base := time.Now().UTC().Truncate(time.Minute).Add(-2 * time.Hour)
	var points []metric.Point
	for entity := 0; entity < 3; entity++ {
		for metricIndex, name := range loadRecordMetricNames {
			for sample := 0; sample < 2; sample++ {
				points = append(points, metric.Point{MetricName: name, EntityID: fmt.Sprintf("batch-node-%d", entity), Timestamp: base.Add(time.Duration(sample+1) * time.Second), Value: float64(100*entity + 10*metricIndex + sample)})
			}
		}
	}
	if err := s.WriteBatch(ctx, points); err != nil {
		t.Fatal(err)
	}
	selects.Store(0)
	records, err := GetRecordsByTime(ctx, base, base.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records=%#v, want distinct nodes", records)
	}
	for i, record := range records {
		if record.Client != fmt.Sprintf("batch-node-%d", i) || record.Cpu != float32(100*i)+0.5 || record.NetTotalUp != int64(100*i+81) || record.TrafficUp != int64(200*i+201) {
			t.Fatalf("lost entity or aggregation semantics: %#v", record)
		}
	}
	if n := selects.Load(); n > 8 {
		t.Fatalf("legacy all-node query prepared %d SELECTs, want <=8 via shared SeriesBatch", n)
	}
}
