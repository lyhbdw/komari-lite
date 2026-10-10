package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// All fixture writes go to an isolated in-memory main DB and a temporary metric
// DB. No application lifecycle, scheduler, or production database is started.
func queryBudgetFixture(t *testing.T) (*gorm.DB, *metric.Store) {
	t.Helper()
	if metricstore.GetStore() != nil {
		t.Fatal("metric store leaked from another test")
	}
	mainDB := dbcore.OpenTestDB(t).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	sqlDB, err := mainDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := mainDB.AutoMigrate(&models.Client{}, &models.PingTask{}); err != nil {
		t.Fatal(err)
	}
	dbcore.SwapInstance(t, mainDB)
	clients.InvalidateClientsCache()
	t.Cleanup(clients.InvalidateClientsCache)
	if err := config.SetDb(mainDB); err != nil {
		t.Fatal(err)
	}
	if err := mainDB.Create(&config.ConfigItem{Key: "metric_db_dsn", Value: `"` + filepath.Join(t.TempDir(), "metrics.db") + `"`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := metricstore.CloseStoreContext(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return mainDB, metricstore.GetStore()
}

func queryBudgetCall(ctx context.Context, method string, params any) *rpc.JsonRpcResponse {
	return Dispatch(ctx, nil, &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, ID: 1, Method: method, Params: params})
}

func TestQueryBudgetRawMaxPointsPreservesAggregation(t *testing.T) {
	_, store := queryBudgetFixture(t)
	base := time.Now().UTC().Truncate(time.Minute).Add(-2 * time.Minute)
	points := make([]metric.Point, 30)
	for i := range points {
		value := 1.0
		if i == 13 {
			value = 999 // A spike that even selection would silently discard.
		}
		points[i] = metric.Point{MetricName: metricstore.MetricCPU, EntityID: "budget-node", Timestamp: base.Add(time.Duration(i) * time.Second), Value: value}
	}
	if err := store.WriteBatch(context.Background(), points); err != nil {
		t.Fatal(err)
	}
	for _, aggregation := range []string{"max", "sum", "avg"} {
		t.Run(aggregation, func(t *testing.T) {
			resp := queryBudgetCall(context.Background(), "public:queryMetrics", map[string]any{
				"metric_key": metricstore.MetricCPU, "entity_id": "budget-node", "start": base, "end": base.Add(30 * time.Second), "max_points": 5, "aggregation": aggregation,
			})
			if resp.Error != nil {
				t.Fatal(resp.Error)
			}
			series := resp.Result.(map[string]any)["series"].([]publicMetricSeries)
			if len(series) != 1 || len(series[0].Points) > 5 {
				t.Fatalf("max_points=5 returned %+v", series)
			}
			count, max, sum := 0, 0.0, 0.0
			for _, p := range series[0].Points {
				count += p.Count
				if p.Value != nil {
					if *p.Value > max {
						max = *p.Value
					}
					sum += *p.Value
				}
			}
			if !series[0].Downsampled || count != 30 {
				t.Fatalf("aggregation discarded samples: %+v", series[0])
			}
			if aggregation == "max" && max != 999 || aggregation == "sum" && sum != 1028 || aggregation == "avg" && sum != 1028.0/30 {
				t.Fatalf("aggregation %s changed values: max=%v sum=%v", aggregation, max, sum)
			}
		})
	}
}

func TestQueryBudgetHistoryReceivesServerDeadline(t *testing.T) {
	mainDB, _ := queryBudgetFixture(t)
	var deadline time.Time
	if err := mainDB.Callback().Query().Before("gorm:query").Register(t.Name(), func(db *gorm.DB) { deadline, _ = db.Statement.Context.Deadline() }); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	resp := queryBudgetCall(context.Background(), "public:queryMetrics", map[string]any{"metric_key": metricstore.MetricCPU, "entity_id": "node"})
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	if deadline.IsZero() || deadline.Sub(before) > 11*time.Second {
		t.Fatalf("history received no bounded server deadline: %s", deadline)
	}
}

func TestQueryBudgetHistoryConcurrencyIsBounded(t *testing.T) {
	mainDB, _ := queryBudgetFixture(t)
	sqlDB, _ := mainDB.DB()
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer conn.Close()
	done := make(chan *rpc.JsonRpcResponse, 3)
	for i := 0; i < 3; i++ {
		go func() {
			done <- queryBudgetCall(ctx, "public:queryMetrics", map[string]any{"metric_key": metricstore.MetricCPU, "entity_id": "node"})
		}()
	}
	rejected := false
	select {
	case resp := <-done:
		rejected = resp.Error != nil && resp.Error.Code == rpc.Unavailable
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	conn.Close()
	remaining := 3
	if rejected {
		remaining--
	}
	for i := 0; i < remaining; i++ {
		<-done
	}
	if !rejected {
		t.Fatal("three concurrent heavy queries were admitted; expected two-slot gate")
	}
	// A cancelled query must release its slot.
	resp := queryBudgetCall(context.Background(), "public:queryMetrics", map[string]any{"metric_key": metricstore.MetricCPU, "entity_id": "node"})
	if resp.Error != nil {
		t.Fatalf("query slot leaked after cancellation: %v", resp.Error)
	}
}

func TestQueryBudgetCancelledDispatchDoesNotInvokeHandler(t *testing.T) {
	method := "common:" + t.Name()
	called := false
	if err := rpc.Register(method, func(context.Context, *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) { called = true; return true, nil }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp := queryBudgetCall(ctx, method, nil)
	if resp.Error == nil || resp.Error.Code != rpc.Cancelled || called {
		t.Fatalf("cancelled request executed handler: response=%+v called=%v", resp, called)
	}
}

func TestQueryBudgetDeadlineCancelsBlockedHistoryLookup(t *testing.T) {
	mainDB, _ := queryBudgetFixture(t)
	sqlDB, _ := mainDB.DB()
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan *rpc.JsonRpcResponse, 1)
	go func() {
		done <- queryBudgetCall(ctx, "public:queryMetrics", map[string]any{"metric_key": metricstore.MetricCPU, "entity_id": "node"})
	}()
	select {
	case resp := <-done:
		if resp.Error == nil || resp.Error.Code != rpc.DeadlineExceeded {
			t.Fatalf("deadline not propagated: %+v", resp.Error)
		}
	case <-time.After(300 * time.Millisecond):
		// Unblock the buggy path before failing, so RED leaves no goroutine.
		conn.Close()
		<-done
		t.Fatal("history lookup ignored request deadline")
	}
}

func TestQueryBudgetOutputReservationRejectsBeforeHistoryRead(t *testing.T) {
	_, store := queryBudgetFixture(t)
	// A closed metric store exposes accidental history reads; shape rejection
	// must happen first, independently of whether matching samples exist.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 128; i++ {
		ids = append(ids, fmt.Sprintf("node-%d", i))
	}
	resp := queryBudgetCall(context.Background(), "public:queryMetrics", map[string]any{"metric_key": metricstore.MetricCPU, "entity_ids": ids, "max_points": 5000})
	if resp.Error == nil || resp.Error.Code != rpc.InvalidParams || !strings.Contains(resp.Error.Message, "budget") {
		t.Fatalf("large output shape was not rejected before history read: %+v", resp.Error)
	}
}

func TestQueryBudgetPingMaxPointsIsCapped(t *testing.T) {
	_, _ = queryBudgetFixture(t)
	end := time.Now().UTC()
	params := map[string]any{"entity_id": "node", "start": end.Add(-100 * time.Hour), "end": end, "max_points": 5000}
	a := queryBudgetCall(context.Background(), "public:getPingMetricStats", params)
	params["max_points"] = 1_000_000_000
	b := queryBudgetCall(context.Background(), "public:getPingMetricStats", params)
	if a.Error != nil || b.Error != nil {
		t.Fatalf("ping stats errors: %v %v", a.Error, b.Error)
	}
	if a.Result.(publicPingMetricStatsResponse).IntervalSeconds != b.Result.(publicPingMetricStatsResponse).IntervalSeconds {
		t.Fatalf("oversized ping max_points bypassed cap: %v vs %v", a.Result, b.Result)
	}
	params["max_points"] = -1
	c := queryBudgetCall(context.Background(), "public:getPingMetricStats", params)
	if c.Error == nil || c.Error.Code != rpc.InvalidParams {
		t.Fatalf("negative max_points accepted: %+v", c)
	}
}

func TestQueryBudgetExpandedEntitiesAreRevalidated(t *testing.T) {
	mainDB, _ := queryBudgetFixture(t)
	for i := 0; i < 129; i++ {
		if err := mainDB.Create(&models.Client{UUID: fmt.Sprintf("node-%03d", i), Token: fmt.Sprintf("synthetic-%03d", i)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	clients.InvalidateClientsCache()
	for _, method := range []string{"public:queryMetrics", "public:getPingMetricStats", "common:getRecords"} {
		resp := queryBudgetCall(context.Background(), method, map[string]any{"metric_key": metricstore.MetricCPU})
		if resp.Error == nil || resp.Error.Code != rpc.InvalidParams {
			t.Errorf("%s accepted expanded 129 entities: %+v", method, resp.Error)
		}
	}
}

func TestQueryBudgetExplicitWindowsAreClampedAtEveryHistoryEntry(t *testing.T) {
	_, _ = queryBudgetFixture(t)
	end := time.Now().UTC()
	start := end.AddDate(-3, 0, 0)
	for _, method := range []string{"public:queryMetrics", "public:getPingMetricStats", "common:getRecords"} {
		t.Run(method, func(t *testing.T) {
			resp := queryBudgetCall(context.Background(), method, map[string]any{"metric_key": metricstore.MetricCPU, "entity_id": "node", "uuid": "node", "start": start, "end": end})
			if resp.Error != nil {
				t.Fatal(resp.Error)
			}
			var gotStart, gotEnd time.Time
			switch v := resp.Result.(type) {
			case map[string]any:
				gotStart, gotEnd = v["start"].(time.Time), v["end"].(time.Time)
			case publicPingMetricStatsResponse:
				gotStart, gotEnd = v.Start, v.End
			default:
				// Exercise the actual compatibility response rather than a helper.
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				var window struct {
					From time.Time
					To   time.Time
				}
				if err := json.Unmarshal(b, &window); err != nil {
					t.Fatal(err)
				}
				gotStart, gotEnd = window.From, window.To
			}
			if !gotEnd.Equal(end) || gotEnd.Sub(gotStart) > 365*24*time.Hour {
				t.Fatalf("explicit window bypassed budget: %s .. %s", gotStart, gotEnd)
			}
		})
	}
}
