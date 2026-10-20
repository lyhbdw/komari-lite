package agent

import (
	"reflect"
	"testing"
	"time"

	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func TestEnqueueV2WrappersPreserveQueueSemantics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  string
		ttl     time.Duration
		params  any
		enqueue func(string) v2.Event
	}{
		{"ping", v2.MethodAgentPing, 3 * time.Second, v2.PingParams{TaskID: 7, Target: "localhost"}, func(id string) v2.Event { return EnqueueV2Ping(id, v2.PingParams{TaskID: 7, Target: "localhost"}) }},
		{"update", v2.MethodAgentUpdate, 30 * time.Minute, v2.UpdateParams{Version: "test"}, func(id string) v2.Event { return EnqueueV2Update(id, v2.UpdateParams{Version: "test"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupEventQueueTestState(t)
			v2EventMu.Lock()
			q := getV2EventQueueLocked("node")
			q.events = []v2.Event{{ID: "expired", Method: "other", ExpiresAt: time.Now().Add(-time.Second)}}
			signal := q.signal
			v2EventMu.Unlock()
			before := time.Now().UTC()
			first := tc.enqueue("node")
			if first.ID == "" || first.Method != tc.method || !reflect.DeepEqual(first.Params, tc.params) || first.ExpiresAt.Sub(first.CreatedAt) != tc.ttl || first.CreatedAt.Before(before) || first.CreatedAt.Location() != time.UTC {
				t.Fatalf("incorrect event: %#v", first)
			}
			select {
			case <-signal:
			default:
				t.Fatal("enqueue did not wake waiters")
			}
			v2EventMu.Lock()
			nextSignal := q.signal
			v2EventMu.Unlock()
			select {
			case <-nextSignal:
				t.Fatal("replacement signal already closed")
			default:
			}
			second := tc.enqueue("node")
			events := TakeV2Events("node", nil, 0)
			if len(events) != 1 || events[0].ID != second.ID || first.ID == second.ID {
				t.Fatalf("expiry/coalescing mismatch: %#v", events)
			}

			// Unique ping tasks fill the queue; either wrapper must evict oldest.
			DeleteV2EventQueue("node")
			var oldest v2.Event
			for i := 0; i < v2EventQueueLimit; i++ {
				e := EnqueueV2Ping("node", v2.PingParams{TaskID: uint(i + 100)})
				if i == 0 {
					oldest = e
				}
			}
			last := tc.enqueue("node")
			events = TakeV2Events("node", nil, 0)
			if len(events) != v2EventQueueLimit || events[0].ID == oldest.ID || events[len(events)-1].ID != last.ID {
				t.Fatal("queue capacity/order mismatch")
			}
		})
	}
}

func TestEnqueueV2CoalescingBoundaries(t *testing.T) {
	setupEventQueueTestState(t)
	first := EnqueueV2Ping("node", v2.PingParams{TaskID: 1})
	EnqueueV2Ping("node", v2.PingParams{TaskID: 2})
	EnqueueV2Ping("node", v2.PingParams{})
	EnqueueV2Ping("node", v2.PingParams{})
	EnqueueV2Update("node", v2.UpdateParams{Version: "old"})
	latest := EnqueueV2Update("node", v2.UpdateParams{Version: "new"})
	events := TakeV2Events("node", nil, 0)
	if len(events) != 5 || events[0].ID != first.ID || events[4].ID != latest.ID {
		t.Fatalf("wrong coalescing boundaries: %#v", events)
	}
}

func TestDispatchV2AdmissionRemainsDifferent(t *testing.T) {
	setupEventQueueTestState(t)
	setupPresenceTestState(t)
	if DispatchPing("offline", v2.PingParams{}) {
		t.Fatal("unmarked ping admitted")
	}
	if len(TakeV2Events("offline", nil, 0)) != 0 {
		t.Fatal("unmarked ping queued")
	}
	if !DispatchV2Update("offline", v2.UpdateParams{}) {
		t.Fatal("offline update rejected")
	}
	MarkV2Client("v2")
	if !DispatchPing("v2", v2.PingParams{}) {
		t.Fatal("marked ping rejected")
	}
	if len(TakeV2Events("offline", nil, 0)) != 1 || len(TakeV2Events("v2", nil, 0)) != 1 {
		t.Fatal("admitted dispatch not queued")
	}
}
