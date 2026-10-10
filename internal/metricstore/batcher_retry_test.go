package metricstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func newRetryTestWorker(t *testing.T, capacity int) *reportBatchWorker {
	t.Helper()
	w := &reportBatchWorker{
		queue: make(chan v2.Report, capacity), pingQueue: make(chan models.PingRecord, capacity),
		requests: make(chan reportBatchRequest, 1), done: make(chan struct{}),
	}
	go w.run()
	t.Cleanup(func() {
		select {
		case <-w.done:
			return
		default:
		}
		if err := retryTestFlush(t, w, true); err != nil {
			t.Errorf("cleanup flush: %v", err)
		}
	})
	return w
}

func retryTestFlush(t *testing.T, w *reportBatchWorker, stop bool) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := reportBatchRequest{ctx: ctx, done: make(chan error, 1), stop: stop}
	select {
	case w.requests <- r:
	case <-ctx.Done():
		t.Fatal("worker did not accept flush")
	}
	select {
	case err := <-r.done:
		return err
	case <-ctx.Done():
		t.Fatal("worker did not finish flush")
		return ctx.Err()
	}
}

func TestBatcherRetryAppliesTotalBackpressure(t *testing.T) {
	ctx := context.Background()
	s := useReportTestStore(t, nil)
	w := newRetryTestWorker(t, 8)
	base := time.Now().UTC().Truncate(time.Second)
	acceptedReports, acceptedPings := 0, 0
	for round := 0; round < 20; round++ {
		for i := 0; i < 2; i++ {
			err := w.enqueue(ctx, v2.Report{UUID: "retry-node", UpdatedAt: base.Add(time.Duration(acceptedReports) * time.Millisecond), CPU: v2.CPUReport{Usage: float64(acceptedReports)}})
			if err == nil {
				acceptedReports++
			} else if !errors.Is(err, ErrReportBatchQueueFull) {
				t.Fatalf("enqueue report: %v", err)
			}
			err = w.enqueuePing(ctx, models.PingRecord{Client: "retry-node", TaskId: 7, Time: base.Add(time.Duration(acceptedPings) * time.Millisecond), Value: acceptedPings})
			if err == nil {
				acceptedPings++
			} else if !errors.Is(err, ErrPingBatchQueueFull) {
				t.Fatalf("enqueue ping: %v", err)
			}
		}
		storeMu.Lock()
		store = nil // Persistent unavailable-store failure, no disk access.
		storeMu.Unlock()
		err := retryTestFlush(t, w, false)
		storeMu.Lock()
		store = s
		storeMu.Unlock()
		if err == nil {
			t.Fatal("expected write failure")
		}
		if acceptedReports > cap(w.queue) || acceptedPings > cap(w.pingQueue) {
			t.Fatalf("accepted reports=%d pings=%d exceed total capacity=8 after failed flush %d", acceptedReports, acceptedPings, round)
		}
	}
	if acceptedReports != 8 || acceptedPings != 8 {
		t.Fatalf("accepted reports=%d pings=%d, want full budget", acceptedReports, acceptedPings)
	}
	if err := retryTestFlush(t, w, false); err != nil {
		t.Fatal(err)
	}
	want := make([]float64, 8)
	for i := range want {
		want[i] = float64(i)
	}
	assertMetricValues(t, s, MetricCPU, "retry-node", base.Add(-time.Second), base.Add(time.Second), want)
	assertMetricValues(t, s, MetricPingLatency, "retry-node", base.Add(-time.Second), base.Add(time.Second), want)
	if err := w.enqueue(ctx, v2.Report{UUID: "retry-node", UpdatedAt: base.Add(time.Second)}); err != nil {
		t.Fatalf("successful retry did not release budget: %v", err)
	}
}
