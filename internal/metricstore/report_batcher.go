package metricstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	logger "github.com/lyhbdw/komari-lite/utils/log"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

type reportTrafficState struct {
	mu sync.Mutex

	reportTrafficValues
}

type reportTrafficValues struct {
	initialized bool
	timestamp   time.Time
	lastSeen    time.Time
	hasUp       bool
	totalUp     int64
	hasDown     bool
	totalDown   int64
}

var reportTrafficStates sync.Map

const (
	reportBatchInterval     = 3 * time.Second
	reportBatchQueueSize    = 4096
	pingBatchMaxRecords     = 512
	reportBatchMaxReports   = 512
	reportBatchWriteTimeout = 10 * time.Second

	// reportTrafficStatePruneInterval controls how often the batcher prunes
	// idle traffic-counter states; reportTrafficStateIdleTTL is how long an
	// agent may stay silent before its state is dropped. The counter is
	// restorable from persisted data (latestReportCounter), so dropping an
	// idle state only costs one restore query on the agent's next report.
	reportTrafficStatePruneInterval = time.Hour
	reportTrafficStateIdleTTL       = 24 * time.Hour
)

var (
	reportBatcherMu sync.Mutex
	reportBatcher   *reportBatchWorker
)

var (
	ErrReportBatchQueueFull = errors.New("metric report batch queue is full")
	ErrReportBatchStopped   = errors.New("metric report batcher is stopped")
	ErrPingBatchQueueFull   = errors.New("ping batch queue is full")
)

type reportBatchRequest struct {
	ctx  context.Context
	done chan error
	stop bool
}

type reportBatchWorker struct {
	mu        sync.Mutex
	queue     chan v2.Report
	pingQueue chan models.PingRecord
	requests  chan reportBatchRequest
	done      chan struct{}
	stopping  bool
	// Budget includes queued, pending and in-flight items; drain does not free it.
	reportsOutstanding int
	pingsOutstanding   int
}

// StartReportBatcher starts the shared report writer. Exact samples are kept in
// the short raw window while the same writes update in-memory minute buckets.
func StartReportBatcher() {
	reportBatcherMu.Lock()
	defer reportBatcherMu.Unlock()
	if reportBatcher != nil {
		return
	}
	worker := &reportBatchWorker{
		queue:     make(chan v2.Report, reportBatchQueueSize),
		pingQueue: make(chan models.PingRecord, reportBatchQueueSize),
		requests:  make(chan reportBatchRequest, 1),
		done:      make(chan struct{}),
	}
	reportBatcher = worker
	go worker.run()
}

// StopReportBatcher stops the report writer after flushing all queued reports.
func StopReportBatcher(ctx context.Context) error {
	reportBatcherMu.Lock()
	worker := reportBatcher
	if worker == nil {
		reportBatcherMu.Unlock()
		return nil
	}
	worker.mu.Lock()
	worker.stopping = true
	worker.mu.Unlock()
	reportBatcherMu.Unlock()

	request := reportBatchRequest{ctx: ctx, done: make(chan error, 1), stop: true}
	select {
	case worker.requests <- request:
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.done:
		// Failed shutdown flushes retain the worker and pending data for retry.
		if err != nil {
			return err
		}
		<-worker.done
		reportBatcherMu.Lock()
		if reportBatcher == worker {
			reportBatcher = nil
		}
		reportBatcherMu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FlushReportBatch synchronously flushes the current queue. It is useful for
// controlled handoff points and deterministic tests; normal operation uses the
// worker ticker and the active mode's flush interval.
func FlushReportBatch(ctx context.Context) error {
	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker == nil {
		return nil
	}
	request := reportBatchRequest{ctx: ctx, done: make(chan error, 1)}
	worker.mu.Lock()
	stopping := worker.stopping
	worker.mu.Unlock()
	if stopping {
		return ErrReportBatchStopped
	}
	select {
	case worker.requests <- request:
	case <-worker.done:
		return ErrReportBatchStopped
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WriteReport persists one agent report and adds it to in-memory minute
// summaries using the same server receive time. Traffic deltas remain summable
// after rollup.
func WriteReport(ctx context.Context, report v2.Report) (v2.Report, error) {
	if report.UUID == "" {
		return v2.Report{}, fmt.Errorf("report UUID is required")
	}
	if report.UpdatedAt.IsZero() {
		return v2.Report{}, fmt.Errorf("report receive time is required")
	}
	report.UpdatedAt = report.UpdatedAt.UTC()
	if GetStore() == nil {
		return v2.Report{}, fmt.Errorf("metric store not enabled")
	}

	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker != nil {
		if err := worker.enqueue(ctx, report); err != nil {
			return v2.Report{}, err
		}
		return report, nil
	}

	saved, err := writeReportBatch(ctx, []v2.Report{report})
	if err != nil {
		return v2.Report{}, err
	}
	return saved[0], nil
}

func (w *reportBatchWorker) enqueue(ctx context.Context, report v2.Report) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping {
		return ErrReportBatchStopped
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.reportsOutstanding >= cap(w.queue) {
		return ErrReportBatchQueueFull
	}
	select {
	case w.queue <- report:
		w.reportsOutstanding++
		return nil
	default:
		return ErrReportBatchQueueFull
	}
}

func (w *reportBatchWorker) run() {
	ticker := time.NewTicker(reportBatchInterval)
	defer ticker.Stop()
	pruneTicker := time.NewTicker(reportTrafficStatePruneInterval)
	defer pruneTicker.Stop()

	var pending []v2.Report
	var pendingPings []models.PingRecord
	for {
		select {
		case request := <-w.requests:
			err := errors.Join(
				w.flushReports(request.ctx, &pending),
				w.flushPings(request.ctx, &pendingPings),
			)
			if request.stop {
				if err != nil {
					logger.Errorf("metricstore", "failed to flush metric report batch during shutdown: %v", err)
					request.done <- err
					continue
				}
				close(w.done)
				request.done <- err
				return
			}
			request.done <- err
		case <-pruneTicker.C:
			pruneReportTrafficStates(time.Now())
		case <-ticker.C:
			if err := w.flushPings(context.Background(), &pendingPings); err != nil {
				logger.Errorf("metricstore", "failed to flush ping batch: %v", err)
			}
			if err := w.flushReports(context.Background(), &pending); err != nil {
				logger.Errorf("metricstore", "failed to flush metric report batch: %v", err)
			}
		}
	}
}

func (w *reportBatchWorker) enqueuePing(ctx context.Context, record models.PingRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping {
		return ErrReportBatchStopped
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.pingsOutstanding >= cap(w.pingQueue) {
		return ErrPingBatchQueueFull
	}
	select {
	case w.pingQueue <- record:
		w.pingsOutstanding++
		return nil
	default:
		return ErrPingBatchQueueFull
	}
}

// Retry pending data before draining again. Only successful chunks release
// budget; errors retain data without freeing channel capacity for new items.
func (w *reportBatchWorker) flushReports(ctx context.Context, pending *[]v2.Report) error {
	for pass := 0; pass < 2; pass++ {
		before := len(*pending)
		err := writePendingReports(ctx, pending)
		w.mu.Lock()
		w.reportsOutstanding -= before - len(*pending)
		w.mu.Unlock()
		if err != nil || pass == 1 {
			return err
		}
		*pending = drainReportQueue(w.queue, reportBatchQueueSize)
	}
	return nil
}

func (w *reportBatchWorker) flushPings(ctx context.Context, pending *[]models.PingRecord) error {
	for pass := 0; pass < 2; pass++ {
		before := len(*pending)
		err := writePendingPingRecords(ctx, pending)
		w.mu.Lock()
		w.pingsOutstanding -= before - len(*pending)
		w.mu.Unlock()
		if err != nil || pass == 1 {
			return err
		}
		*pending = drainPingQueue(w.pingQueue, reportBatchQueueSize)
	}
	return nil
}

// pruneReportTrafficStates drops traffic-counter states for agents that have
// not reported within the idle TTL, so agents that disappear do not leak
// entries forever. A returning agent re-initializes its counter from persisted
// data (latestReportCounter), so pruning is lossless apart from one query.
func pruneReportTrafficStates(now time.Time) {
	cutoff := now.Add(-reportTrafficStateIdleTTL)
	reportTrafficStates.Range(func(key, value any) bool {
		state, ok := value.(*reportTrafficState)
		if !ok {
			reportTrafficStates.Delete(key)
			return true
		}
		state.mu.Lock()
		idle := !state.lastSeen.IsZero() && state.lastSeen.Before(cutoff)
		state.mu.Unlock()
		if idle {
			reportTrafficStates.Delete(key)
		}
		return true
	})
}

func drainReportQueue(queue <-chan v2.Report, limit int) []v2.Report {
	if limit <= 0 {
		return nil
	}
	capacity := len(queue)
	if capacity > limit {
		capacity = limit
	}
	if capacity == 0 {
		return nil
	}
	reports := make([]v2.Report, 0, capacity)
	for len(reports) < limit {
		select {
		case report := <-queue:
			reports = append(reports, report)
		default:
			return reports
		}
	}
	return reports
}

func drainPingQueue(queue <-chan models.PingRecord, limit int) []models.PingRecord {
	if limit <= 0 {
		return nil
	}
	capacity := len(queue)
	if capacity > limit {
		capacity = limit
	}
	if capacity == 0 {
		return nil
	}
	records := make([]models.PingRecord, 0, capacity)
	for len(records) < limit {
		select {
		case record := <-queue:
			records = append(records, record)
		default:
			return records
		}
	}
	return records
}

func writePendingReports(ctx context.Context, pending *[]v2.Report) error {
	for len(*pending) > 0 {
		batchSize := len(*pending)
		if batchSize > reportBatchMaxReports {
			batchSize = reportBatchMaxReports
		}
		writeCtx, cancel := context.WithTimeout(ctx, reportBatchWriteTimeout)
		_, err := writeReportBatch(writeCtx, (*pending)[:batchSize])
		cancel()
		if err != nil {
			return err
		}
		clear((*pending)[:batchSize])
		*pending = (*pending)[batchSize:]
	}
	*pending = nil
	return nil
}

func writePendingPingRecords(ctx context.Context, pending *[]models.PingRecord) error {
	for len(*pending) > 0 {
		batchSize := len(*pending)
		if batchSize > pingBatchMaxRecords {
			batchSize = pingBatchMaxRecords
		}
		writeCtx, cancel := context.WithTimeout(ctx, reportBatchWriteTimeout)
		err := writePingRecords(writeCtx, (*pending)[:batchSize])
		cancel()
		if err != nil {
			return err
		}
		clear((*pending)[:batchSize])
		*pending = (*pending)[batchSize:]
	}
	*pending = nil
	return nil
}

func writeReportBatch(ctx context.Context, reports []v2.Report) ([]v2.Report, error) {
	if len(reports) == 0 {
		return nil, nil
	}
	if err := storeOperations.AcquireShared(ctx); err != nil {
		return nil, fmt.Errorf("wait for metric store operation before writing reports: %w", err)
	}
	defer storeOperations.ReleaseShared()

	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	prepared := make([]v2.Report, len(reports))
	copy(prepared, reports)
	points := make([]metric.Point, 0, len(reports)*20)
	pendingStates := make(map[*reportTrafficState]reportTrafficValues)
	for i, report := range prepared {
		stateValue, _ := reportTrafficStates.LoadOrStore(report.UUID, &reportTrafficState{})
		state := stateValue.(*reportTrafficState)
		values, ok := pendingStates[state]
		if !ok {
			state.mu.Lock()
			values = state.reportTrafficValues
			state.mu.Unlock()
		}
		if !values.initialized {
			totalUp, hasUp, err := latestReportCounter(ctx, s, MetricNetTotalUp, report.UUID, report.UpdatedAt)
			if err != nil {
				logger.Errorf("metricstore", "failed to restore previous upload counter for %s: %v", report.UUID, err)
			} else {
				values.totalUp = totalUp
				values.hasUp = hasUp
			}
			totalDown, hasDown, err := latestReportCounter(ctx, s, MetricNetTotalDown, report.UUID, report.UpdatedAt)
			if err != nil {
				logger.Errorf("metricstore", "failed to restore previous download counter for %s: %v", report.UUID, err)
			} else {
				values.totalDown = totalDown
				values.hasDown = hasDown
			}
			values.initialized = true
			// Cache only the restored baseline on failure, not tentative counters
			// from this batch. Retrying must use the same timestamp and deltas.
			state.mu.Lock()
			state.reportTrafficValues = values
			state.mu.Unlock()
		}

		if !values.timestamp.IsZero() && !report.UpdatedAt.After(values.timestamp) {
			report.UpdatedAt = values.timestamp.Add(time.Millisecond)
		}
		trafficUp := int64(0)
		if values.hasUp {
			trafficUp = TrafficCounterDelta(report.Network.TotalUp, values.totalUp)
		}
		trafficDown := int64(0)
		if values.hasDown {
			trafficDown = TrafficCounterDelta(report.Network.TotalDown, values.totalDown)
		}
		points = append(points, reportMetricPoints(report, trafficUp, trafficDown)...)
		values.timestamp = report.UpdatedAt
		values.hasUp = true
		values.totalUp = report.Network.TotalUp
		values.hasDown = true
		values.totalDown = report.Network.TotalDown
		values.lastSeen = time.Now()
		pendingStates[state] = values
		prepared[i] = report
	}

	if err := s.WriteBatch(ctx, points); err != nil {
		return nil, err
	}
	// Advance receive timestamps and counters only after acceptance succeeds.
	// WriteBatch may already retain raw/hot points on a persistence failure;
	// retrying the same point identity replaces them instead of adding samples.
	for state, values := range pendingStates {
		state.mu.Lock()
		state.reportTrafficValues = values
		state.mu.Unlock()
	}

	return prepared, nil
}
