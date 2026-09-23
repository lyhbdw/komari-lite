package metric

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// newMemStore opens an isolated in-memory store for tests.
//
// newMemStore 打开一个用于测试的隔离内存 Store。
func newMemStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), SQLite("file:imp-test?mode=memory&cache=shared"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// TestCreateMetricRejectsDuplicate verifies create-only metric semantics.
//
// TestCreateMetricRejectsDuplicate 验证 CreateMetric 遇到重复指标时会拒绝。
func TestCreateMetricRejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	def := Definition{Name: "dup.metric", Type: TypeGauge, RetentionDays: 30}
	if err := s.CreateMetric(ctx, def); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := s.CreateMetric(ctx, def)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists on duplicate, got %v", err)
	}
}

// TestUpsertMetricOverwrites verifies upsert updates mutable definition fields.
//
// TestUpsertMetricOverwrites 验证 UpsertMetric 会更新指标定义的可变字段。
func TestUpsertMetricOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "m", Type: TypeGauge, Unit: "ms", RetentionDays: 30}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.UpsertMetric(ctx, Definition{Name: "m", Type: TypeCounter, Unit: "count", RetentionDays: 30}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.GetMetric(ctx, "m")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != TypeCounter || got.Unit != "count" {
		t.Fatalf("upsert did not overwrite: %#v", got)
	}
}

// TestTagFilterPushdownWithPaging verifies tag filtering happens before paging.
//
// TestTagFilterPushdownWithPaging 验证标签过滤会先于分页在 SQL 中执行。
func TestTagFilterPushdownWithPaging(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "req", Type: TypeCounter, RetentionDays: 30}); err != nil {
		t.Fatalf("create: %v", err)
	}
	base := time.Now().UTC().Truncate(time.Minute)
	var batch []Point
	for i := 0; i < 6; i++ {
		env := "prod"
		if i%2 == 1 {
			env = "stage"
		}
		batch = append(batch, Point{
			MetricName: "req", EntityID: "n1",
			Timestamp: base.Add(time.Duration(i) * 5 * time.Second),
			Value:     float64(i),
			Tags:      map[string]string{"env": env},
		})
	}
	if err := s.WriteBatch(ctx, batch); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 3 prod points (i=0,2,4); with Limit=2 the SQL LIMIT must apply to the
	// tag-filtered set, returning the first two prod points in time order.
	got, err := s.Query(ctx, Query{
		MetricName: "req", EntityID: "n1",
		Start: base, End: base.Add(time.Minute),
		Tags:  map[string]string{"env": "prod"},
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 paged prod points, got %d", len(got))
	}
	if got[0].Value != 0 || got[1].Value != 2 {
		t.Fatalf("unexpected paged values: %#v", got)
	}
}


// TestCounterRateHandlesReset verifies reset-aware counter rate calculation.
//
// TestCounterRateHandlesReset 验证计数器重置时速率计算仍然稳定。
func TestCounterRateHandlesReset(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute)
	// Counter goes 0 -> 10 -> reset -> 5; naive (last-first)/sec would give a
	// positive-but-wrong 5/30s. Correct counter rate sums positive deltas:
	// (10-0) + (5-0 after reset) = 15 over 30s = 0.5/s.
	pts := []Point{
		{Timestamp: base, Value: 0},
		{Timestamp: base.Add(10 * time.Second), Value: 10},
		{Timestamp: base.Add(20 * time.Second), Value: 0},
		{Timestamp: base.Add(30 * time.Second), Value: 5},
	}
	rate := counterRate(pts)
	if rate != 0.5 {
		t.Fatalf("expected reset-aware rate 0.5/s, got %v", rate)
	}
}

// TestAlignTimeNegativeTimestamp verifies pre-epoch bucket alignment.
//
// TestAlignTimeNegativeTimestamp 验证 Unix epoch 之前的时间也能正确对齐桶。
func TestAlignTimeNegativeTimestamp(t *testing.T) {
	interval := time.Minute
	// 30s before the epoch should align down to -60s, not up to 0.
	tm := time.Unix(-30, 0).UTC()
	got := alignTime(tm, interval)
	want := time.Unix(-60, 0).UTC()
	if !got.Equal(want) {
		t.Fatalf("alignTime negative: got %v want %v", got, want)
	}
}



// TestStdDevPopMatchesCalculateStats verifies population standard deviation.
//
// TestStdDevPopMatchesCalculateStats 验证总体标准差与统计摘要一致。
func TestStdDevPopMatchesCalculateStats(t *testing.T) {
	base := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	var pts []Point
	for i, v := range []float64{10, 20, 30, 40, 50} {
		pts = append(pts, Point{Timestamp: base.Add(time.Duration(i) * time.Minute), Value: v})
	}
	st, err := CalculateStats(pts)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	got, _ := aggregateValue(pts, AggStdDev)
	if math.Abs(got-st.StdDev) > 1e-9 {
		t.Fatalf("AggStdDev %v != Stats.StdDev %v", got, st.StdDev)
	}
	// Known value: population stddev of 10..50 step 10 is sqrt(200) ~= 14.142135.
	if math.Abs(got-14.142135623730951) > 1e-9 {
		t.Fatalf("unexpected population stddev: %v", got)
	}
}


// TestSQLiteReadPoolOpens verifies SQLite read-pool creation.
//
// TestSQLiteReadPoolOpens 验证 SQLite 只读连接池会按配置打开。
func TestSQLiteReadPoolOpens(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "rp")
	store, err := Open(ctx, SQLiteInDir(dir, WithSQLiteReadPool(4)))
	if err != nil {
		t.Fatalf("open with read pool: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if store.readDB == nil {
		t.Fatalf("expected a dedicated read pool to be opened")
	}
	if store.reader() != store.readDB {
		t.Fatalf("reader() should return the dedicated read pool")
	}
	// Round-trip a write (primary) and a read (read pool).
	if err := store.CreateMetric(ctx, Definition{Name: "rp", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatalf("create: %v", err)
	}
	base := time.Now().UTC()
	if err := store.Write(ctx, Point{MetricName: "rp", EntityID: "n1", Timestamp: base, Value: 7}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := store.Query(ctx, Query{MetricName: "rp", EntityID: "n1", Start: base.Add(-time.Minute), End: base.Add(time.Minute), Order: OrderDesc, Limit: 1})
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if len(got) != 1 || got[0].Value != 7 {
		t.Fatalf("read pool round-trip failed: %#v", got)
	}
}

// TestMemoryDSNSkipsReadPool verifies memory SQLite skips read pools.
//
// TestMemoryDSNSkipsReadPool 验证内存 SQLite 不会打开独立读池。
func TestMemoryDSNSkipsReadPool(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, SQLite("file:rp-mem?mode=memory&cache=shared", WithSQLiteReadPool(4)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if store.readDB != nil {
		t.Fatalf("in-memory database must not open a second read pool")
	}
}

// TestWriteBatchAtomicAcrossChunks verifies chunked writes are atomic.
//
// TestWriteBatchAtomicAcrossChunks 验证分块批量写入仍保持整体原子性。
func TestWriteBatchAtomicAcrossChunks(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "atom", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatalf("create: %v", err)
	}
	base := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	// 1500 valid points, then one invalid (missing entity) to force a mid-batch
	// failure on the second chunk. The whole batch must roll back.
	var batch []Point
	for i := 0; i < 1500; i++ {
		batch = append(batch, Point{MetricName: "atom", EntityID: "n1", Timestamp: base.Add(time.Duration(i) * time.Millisecond), Value: float64(i)})
	}
	batch = append(batch, Point{MetricName: "atom", EntityID: "", Timestamp: base.Add(2 * time.Second), Value: 1}) // invalid
	if err := s.WriteBatch(ctx, batch); err == nil {
		t.Fatalf("expected WriteBatch to fail on invalid point")
	}
	// Nothing should have been committed.
	pts, err := s.Query(ctx, Query{MetricName: "atom", EntityID: "n1", Start: base, End: base.Add(time.Hour)})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(pts) != 0 {
		t.Fatalf("expected 0 committed points after rolled-back batch, got %d", len(pts))
	}
}






// TestJSONTagKeyWithSpecialChars verifies JSON tag keys with special characters.
//
// TestJSONTagKeyWithSpecialChars 验证包含特殊字符的标签键可正确查询。
func TestJSONTagKeyWithSpecialChars(t *testing.T) {
	ctx := context.Background()
	s := newMemStore(t)
	if err := s.CreateMetric(ctx, Definition{Name: "tk", Type: TypeGauge, RetentionDays: 30}); err != nil {
		t.Fatalf("create: %v", err)
	}
	base := time.Now().UTC().Truncate(time.Minute)
	// Keys containing a dot, a hyphen, a space, and a quote.
	tags := map[string]string{
		"region.zone": "ap-1",
		"a-b":         "yes",
		"with space":  "ok",
	}
	if err := s.Write(ctx, Point{MetricName: "tk", EntityID: "n1", Timestamp: base, Value: 1, Tags: tags}); err != nil {
		t.Fatalf("write: %v", err)
	}
	// A second point that should NOT match the filter below.
	if err := s.Write(ctx, Point{MetricName: "tk", EntityID: "n1", Timestamp: base.Add(10 * time.Second), Value: 2, Tags: map[string]string{"region.zone": "eu-1"}}); err != nil {
		t.Fatalf("write 2: %v", err)
	}

	for _, tc := range []struct {
		key, val string
		want     int
	}{
		{"region.zone", "ap-1", 1}, // dotted key must match the flat key, not nested
		{"a-b", "yes", 1},          // hyphen
		{"with space", "ok", 1},    // space
		{"region.zone", "eu-1", 1}, // the other point
	} {
		got, err := s.Query(ctx, Query{
			MetricName: "tk", EntityID: "n1",
			Start: base.Add(-time.Second), End: base.Add(time.Hour),
			Tags: map[string]string{tc.key: tc.val},
		})
		if err != nil {
			t.Fatalf("query %s=%s: %v", tc.key, tc.val, err)
		}
		if len(got) != tc.want {
			t.Fatalf("tag %q=%q: expected %d points, got %d", tc.key, tc.val, tc.want, len(got))
		}
	}
}
