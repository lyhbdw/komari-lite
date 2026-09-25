package agent

import (
	"testing"
	"time"

	v2 "github.com/Tumb1er1376/komari-monitor-lite/protocol/v2"
)

func setupEventQueueTestState(t *testing.T) {
	t.Helper()
	v2EventMu.Lock()
	previous := v2EventQueues
	v2EventQueues = make(map[string]*v2EventQueue)
	v2EventMu.Unlock()
	t.Cleanup(func() {
		v2EventMu.Lock()
		v2EventQueues = previous
		v2EventMu.Unlock()
	})
}

func TestZeroTTLEventsExpire(t *testing.T) {
	setupEventQueueTestState(t)

	// 直接构造零 TTL 事件（防御未来不带 TTL 的事件源），入队后按
	// CreatedAt + 默认 TTL 判定过期。
	old := time.Now().UTC().Add(-v2EventDefaultTTL - time.Minute)
	event := v2.Event{
		ID:        "evt-zero-ttl",
		Method:    v2.MethodAgentPing,
		Params:    v2.PingParams{TaskID: 7, Type: "icmp", Target: "1.1.1.1"},
		CreatedAt: old,
	}
	v2EventMu.Lock()
	q := getV2EventQueueLocked("ttl-node")
	q.events = append(q.events, event)
	v2EventMu.Unlock()

	events := TakeV2Events("ttl-node", nil, 8)
	if len(events) != 0 {
		t.Fatalf("zero-TTL event survived past default TTL: %#v", events)
	}
}

func TestDeleteV2EventQueueRemovesQueueAndWakesWaiters(t *testing.T) {
	setupEventQueueTestState(t)

	// Ensure the queue exists with a waiter blocked in WaitV2Events.
	waitDone := make(chan []v2.Event, 1)
	go func() {
		waitDone <- WaitV2Events("gone-node", nil, 30*time.Second)
	}()

	// Give the waiter a moment to enter the wait, then delete the queue.
	time.Sleep(50 * time.Millisecond)
	DeleteV2EventQueue("gone-node")

	select {
	case events := <-waitDone:
		if len(events) != 0 {
			t.Fatalf("expected no events after queue deletion, got %#v", events)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitV2Events blocked forever after its queue was deleted")
	}

	v2EventMu.Lock()
	_, exists := v2EventQueues["gone-node"]
	v2EventMu.Unlock()
	if exists {
		t.Fatal("queue entry survived DeleteV2EventQueue")
	}

	// Deleting a missing queue must be a no-op.
	DeleteV2EventQueue("never-existed")
}
