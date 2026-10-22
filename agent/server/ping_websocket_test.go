package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	pkgflags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
)

func TestWebSocketPingFloodDoesNotCreateWaitingWorkers(t *testing.T) {
	resetRawEventState(t)
	oldGate := pingAdmission
	gate := newPingAdmission(3, func(pingJob) {})
	release := make(chan struct{})
	gate.run = func(pingJob) { <-release }
	pingAdmission = gate
	defer func() {
		close(release)
		deadline := time.Now().Add(time.Second)
		for len(gate.slots) > 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		pingAdmission = oldGate
	}()
	sent := make(chan struct{})
	pong := make(chan struct{})
	upgrader := websocket.Upgrader{}
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()
		c.SetPongHandler(func(string) error { close(pong); return nil })
		for i := 0; i < 1000; i++ {
			if err = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":"flood-%d","method":"agent.ping","params":{"ping_task_id":1,"ping_type":"tcp","ping_target":"127.0.0.1:80"}}`, i))); err != nil {
				t.Error(err)
				return
			}
		}
		c.WriteControl(websocket.PingMessage, []byte("fence"), time.Now().Add(time.Second))
		close(sent)
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		c.ReadMessage()
	}))
	defer controller.Close()
	oldFlags := flags
	flags = &pkgflags.Config{}
	defer func() { flags = oldFlags }()
	conn, err := connectWebSocket("ws" + strings.TrimPrefix(controller.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go handleWebSocketMessages(conn, done)
	defer func() { conn.Close(); <-done }()
	baseline := runtime.NumGoroutine()
	<-sent
	select {
	case <-pong:
	case <-time.After(3 * time.Second):
		t.Fatal("WS flood did not drain")
	}
	if len(gate.slots) != 3 {
		t.Fatalf("workers=%d", len(gate.slots))
	}
	v2AckMu.Lock()
	count := len(v2SeenEvents)
	v2AckMu.Unlock()
	if count != 3 {
		t.Fatalf("accepted/seen=%d, want 3", count)
	}
	if got := runtime.NumGoroutine(); got > baseline+8 {
		t.Fatalf("goroutines grew from %d to %d", baseline, got)
	}
}
