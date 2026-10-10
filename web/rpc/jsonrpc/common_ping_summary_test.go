package jsonrpc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
)

func getPingStatsForNode(uuid string, pingTasks []models.PingTask) map[string]pingStat {
	return getPingStatsForNodeContext(context.Background(), uuid, pingTasks)
}

func preparePingSummaryStore(t *testing.T) {
	t.Helper()
	db := dbcore.OpenTestDB(t)
	if err := config.SetDb(db); err != nil {
		t.Fatal(err)
	}
	if err := config.Set("metric_db_dsn", filepath.Join(t.TempDir(), "metrics.db")); err != nil {
		t.Fatal(err)
	}
	if err := metricstore.InitializeStore(); err != nil {
		t.Fatal(err)
	}
	pingStatsCache.Flush()
	t.Cleanup(func() {
		pingStatsCache.Flush()
		if err := metricstore.CloseStoreContext(context.Background()); err != nil {
			t.Error(err)
		}
	})
}

func TestNodePingSummaryCountsMixedBucketLossAndLatestLoss(t *testing.T) {
	preparePingSummaryStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	for i, latency := range []int{20, 40, -1} {
		if err := metricstore.WritePingRecord(ctx, models.PingRecord{Client: "node", TaskId: 7, Time: base.Add(time.Duration(i) * time.Second), Value: latency}); err != nil {
			t.Fatal(err)
		}
	}
	got := getPingStatsForNode("node", []models.PingTask{{Id: 7, Name: "link", Clients: models.StringArray{"node"}}})["7"]
	if got.Latest != -1 {
		t.Fatalf("latest loss must not reuse an older successful ping: %+v", got)
	}
	if got.Avg != 30 {
		t.Fatalf("successful average = %d, want 30 (loss must not bias latency)", got.Avg)
	}
	if got.Loss < 33.3 || got.Loss > 33.4 {
		t.Fatalf("loss = %g, want one lost probe out of three", got.Loss)
	}
}

func TestNodePingSummaryCacheTracksAssignmentAndNameChanges(t *testing.T) {
	preparePingSummaryStore(t)
	if err := metricstore.WritePingRecord(context.Background(), models.PingRecord{Client: "node", TaskId: 7, Time: time.Now().UTC().Add(-time.Second), Value: 20}); err != nil {
		t.Fatal(err)
	}
	tasks := []models.PingTask{{Id: 7, Name: "old", Clients: models.StringArray{"node"}}}
	if got := getPingStatsForNode("node", tasks)["7"].Name; got != "old" {
		t.Fatal(got)
	}
	tasks[0].Name = "new"
	if got := getPingStatsForNode("node", tasks)["7"].Name; got != "new" {
		t.Fatalf("cached metadata survived task rename: %s", got)
	}
	tasks[0].Clients = nil
	if got := getPingStatsForNode("node", tasks); len(got) != 0 {
		t.Fatalf("removed assignment is still exposed: %+v", got)
	}
}

func TestNodePingSummaryRetainsFullLossAndZeroLatency(t *testing.T) {
	preparePingSummaryStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Minute)
	for _, record := range []models.PingRecord{
		{Client: "node", TaskId: 1, Time: now, Value: 0},
		{Client: "node", TaskId: 1, Time: now.Add(time.Second), Value: 5},
		{Client: "node", TaskId: 2, Time: now, Value: -1},
		{Client: "node", TaskId: 2, Time: now.Add(time.Second), Value: -1},
	} {
		if err := metricstore.WritePingRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	stats := getPingStatsForNode("node", []models.PingTask{
		{Id: 1, Weight: 2, Clients: models.StringArray{"node"}},
		{Id: 2, Weight: 1, Clients: models.StringArray{"node"}},
	})
	if len(stats) != 2 || stats["1"].Min != 0 || stats["2"].Loss != 100 || stats["2"].Latest != -1 || stats["2"].Weight != 1 {
		t.Fatalf("zero/full-loss task summaries were altered: %+v", stats)
	}
}
