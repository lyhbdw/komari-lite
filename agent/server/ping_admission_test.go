package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgflags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
)

func TestPingAdmissionRejectsWithoutACKOrDedup(t *testing.T) {
	resetRawEventState(t)
	old := pingAdmission
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	var wg sync.WaitGroup
	pingAdmission = newPingAdmission(3, func(task pingJob) { defer wg.Done(); started <- struct{}{}; <-release })
	defer func() { close(release); wg.Wait(); pingAdmission = old }()
	params := map[string]interface{}{"ping_task_id": 1, "ping_type": "tcp", "ping_target": "localhost:80"}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		if !processV2Event(nil, v2.MethodAgentPing, params, fmt.Sprint(i)) {
			t.Fatal("normal task rejected")
		}
	}
	for i := 0; i < 3; i++ {
		<-started
	}
	for i := 0; i < 1000; i++ {
		if processV2Event(nil, v2.MethodAgentPing, params, "retry") {
			t.Fatal("saturated admission reported success")
		}
	}
	v2AckMu.Lock()
	_, seen := v2SeenEvents["retry"]
	v2AckMu.Unlock()
	if seen {
		t.Fatal("rejected task recorded as seen")
	}
	response := &v2.Response{Result: []byte(`{"events":[{"id":"retry","method":"agent.ping","params":{"ping_task_id":1,"ping_type":"tcp","ping_target":"localhost:80"}}]}`)}
	processV2ResponseEvents(response)
	if got := snapshotV2AckEventIDs(); len(got) != 0 {
		t.Fatalf("rejected task ACKed: %v", got)
	}
	if len(pingAdmission.slots) != 3 {
		t.Fatal("active bound changed")
	}
}

func TestRejectedPingCanRetrySameEventID(t *testing.T) {
	resetRawEventState(t)
	old := pingAdmission
	var count atomic.Int32
	pingAdmission = newPingAdmission(1, func(pingJob) { count.Add(1) })
	defer func() { pingAdmission = old }()
	pingAdmission.slots <- struct{}{}
	p := map[string]interface{}{"ping_task_id": 1, "ping_type": "tcp", "ping_target": "localhost:80"}
	if processV2Event(nil, v2.MethodAgentPing, p, "same") {
		t.Fatal("full accepted")
	}
	<-pingAdmission.slots
	if !processV2Event(nil, v2.MethodAgentPing, p, "same") {
		t.Fatal("retry rejected")
	}
	deadline := time.Now().Add(time.Second)
	for len(pingAdmission.slots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if count.Load() != 1 {
		t.Fatal("retry did not execute")
	}
	if !processV2Event(nil, v2.MethodAgentPing, p, "same") {
		t.Fatal("accepted duplicate should ACK")
	}
	if count.Load() != 1 {
		t.Fatal("duplicate executed")
	}
}

// Retain the normal three-task, one-minute workload using real HTTP probes and POST results.
func TestNormalThreePingTasksFor60Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("one-minute cadence integration")
	}
	resetRawEventState(t)
	var results atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { results.Add(1); w.Write([]byte(`{"jsonrpc":"2.0"}`)) }))
	defer controller.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer target.Close()
	oldFlags := flags
	flags = &pkgflags.Config{Endpoint: controller.URL, DisableCompression: true}
	defer func() { flags = oldFlags }()
	for round := 0; round < 60; round++ {
		start := time.Now()
		for task := 1; task <= 3; task++ {
			p := map[string]interface{}{"ping_task_id": task, "ping_type": "http", "ping_target": target.URL}
			if !processV2Event(nil, v2.MethodAgentPing, p, fmt.Sprintf("normal-%d-%d", round, task)) {
				t.Fatal("normal workload rejected")
			}
		}
		time.Sleep(time.Until(start.Add(time.Second)))
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(pingAdmission.slots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := results.Load(); got != 180 {
		t.Fatalf("normal minute results=%d, want 180", got)
	}
}
