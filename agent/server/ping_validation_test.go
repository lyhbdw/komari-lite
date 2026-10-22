package server

import (
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"sync/atomic"
	"testing"
	"time"
)

func TestInvalidPingIsNotAdmittedOrDeduplicated(t *testing.T) {
	resetRawEventState(t)
	old := pingAdmission
	var count atomic.Int32
	gate := newPingAdmission(3, func(pingJob) { count.Add(1) })
	pingAdmission = gate
	defer func() {
		deadline := time.Now().Add(time.Second)
		for len(gate.slots) > 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		pingAdmission = old
	}()
	for _, p := range []map[string]interface{}{
		{"ping_task_id": 0, "ping_type": "tcp", "ping_target": "localhost:80"},
		{"ping_task_id": 1, "ping_type": "shell", "ping_target": "localhost:80"},
		{"ping_task_id": 1, "ping_type": "tcp", "ping_target": ""},
	} {
		if processV2Event(nil, v2.MethodAgentPing, p, "invalid") {
			t.Error("invalid ping accepted")
		}
	}
	v2AckMu.Lock()
	_, seen := v2SeenEvents["invalid"]
	v2AckMu.Unlock()
	if seen {
		t.Error("invalid ping recorded as seen")
	}
}
