package utils

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
)

func newPingGroupRunner(tasks []models.PingTask, execute func(context.Context, models.PingTask)) func(context.Context) {
	return newPingGroupRunnerWithGate(tasks, execute, &atomic.Bool{})
}

func TestPingGroupHoldsGateUntilEveryTaskReturns(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	run := newPingGroupRunner([]models.PingTask{{Id: 1}, {Id: 2}}, func(context.Context, models.PingTask) {
		calls.Add(1)
		started <- struct{}{}
		<-release
	})
	done := make(chan struct{})
	go func() { run(context.Background()); close(done) }()
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("group did not start its tasks")
		}
	}
	select {
	case <-done:
		t.Fatal("group returned while child tasks were still executing")
	default:
	}
	run(context.Background()) // An overlapping tick must be skipped.
	if calls.Load() != 2 {
		t.Fatalf("overlapping tick started more tasks: %d", calls.Load())
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("finished group did not release the scheduler callback")
	}
}

func TestPingGroupSkipsCancelledContext(t *testing.T) {
	var calls atomic.Int32
	run := newPingGroupRunner([]models.PingTask{{Id: 1}}, func(context.Context, models.PingTask) { calls.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run(ctx)
	if calls.Load() != 0 {
		t.Fatal("cancelled group dispatched work")
	}
}

func TestPingGroupCanRunAgainAfterCompletion(t *testing.T) {
	var calls atomic.Int32
	run := newPingGroupRunner([]models.PingTask{{Id: 1}}, func(context.Context, models.PingTask) { calls.Add(1) })
	run(context.Background())
	run(context.Background())
	if calls.Load() != 2 {
		t.Fatalf("gate did not reopen: %d tasks", calls.Load())
	}
}
