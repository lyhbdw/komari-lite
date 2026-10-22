package metricstore

import (
	"context"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

// The runtime snapshot must combine the store's in-memory shape, the SQLite
// pool occupancy and the batcher queue depth in one read-only view, because
// those three numbers are what an operator needs to attribute container
// memory: the store maps heap growth to a tier, the pool shows connection
// starvation, and the queue shows ingestion backlog.
func TestRuntimeMemorySnapshotReportsStoreQueuesAndPools(t *testing.T) {
	ctx := context.Background()
	s := useReportTestStore(t, nil)
	base := time.Now().UTC().Truncate(time.Minute)
	if err := s.WriteBatch(ctx, []metric.Point{
		{MetricName: MetricCPU, EntityID: "snap-node", Timestamp: base.Add(10 * time.Second), Value: 12},
		{MetricName: MetricCPU, EntityID: "snap-node", Timestamp: base.Add(20 * time.Second), Value: 18},
	}); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	// A worker that is not running keeps its queue observable instead of
	// draining it, which makes the snapshot deterministic.
	worker := &reportBatchWorker{
		queue:     make(chan v2.Report, 8),
		pingQueue: make(chan models.PingRecord, 8),
		requests:  make(chan reportBatchRequest, 1),
		done:      make(chan struct{}),
	}
	reportBatcherMu.Lock()
	previous := reportBatcher
	reportBatcher = worker
	reportBatcherMu.Unlock()
	t.Cleanup(func() {
		reportBatcherMu.Lock()
		reportBatcher = previous
		reportBatcherMu.Unlock()
	})
	if err := worker.enqueue(ctx, v2.Report{UUID: "snap-node", UpdatedAt: base}); err != nil {
		t.Fatalf("enqueue report: %v", err)
	}
	if err := worker.enqueuePing(ctx, models.PingRecord{Client: "snap-node", TaskId: 1, Time: base}); err != nil {
		t.Fatalf("enqueue ping: %v", err)
	}

	snapshot := TakeRuntimeMemorySnapshot()
	if snapshot.Metric.RawSamples == 0 {
		t.Errorf("snapshot must report retained raw samples: %#v", snapshot.Metric)
	}
	if snapshot.Metric.HotBuckets == 0 {
		t.Errorf("snapshot must report hot buckets: %#v", snapshot.Metric)
	}
	if snapshot.Queues.ReportsQueued != 1 || snapshot.Queues.PingsQueued != 1 {
		t.Errorf("queue depth = %d reports / %d pings, want 1 / 1", snapshot.Queues.ReportsQueued, snapshot.Queues.PingsQueued)
	}
	if snapshot.Pools.MaxOpenConnections != 1 {
		t.Errorf("write pool ceiling = %d, want 1", snapshot.Pools.MaxOpenConnections)
	}
	if snapshot.Go.HeapAllocBytes == 0 {
		t.Errorf("snapshot must report Go heap allocation: %#v", snapshot.Go)
	}
	if snapshot.Go.SoftMemoryLimitBytes <= 0 {
		t.Errorf("snapshot must report a positive soft memory limit: %#v", snapshot.Go)
	}
}

// Without an initialized store the snapshot must still be safe to take: the
// admin endpoint runs before the store opens on a fresh install.
func TestRuntimeMemorySnapshotWithoutStore(t *testing.T) {
	storeMu.Lock()
	previous := store
	store = nil
	storeMu.Unlock()
	t.Cleanup(func() {
		storeMu.Lock()
		store = previous
		storeMu.Unlock()
	})

	snapshot := TakeRuntimeMemorySnapshot()
	if snapshot.Metric != (metric.MemoryStats{}) {
		t.Fatalf("store-less snapshot must zero the metric section: %#v", snapshot.Metric)
	}
	if snapshot.Pools != (metric.DBPoolStats{}) {
		t.Fatalf("store-less snapshot must zero the pool section: %#v", snapshot.Pools)
	}
	if snapshot.Queues.ReportsQueued != 0 || snapshot.Queues.PingsQueued != 0 {
		t.Fatalf("store-less snapshot must zero the queue section: %#v", snapshot.Queues)
	}
	if snapshot.Go.HeapAllocBytes == 0 {
		t.Fatalf("runtime stats must still be reported: %#v", snapshot.Go)
	}
}
