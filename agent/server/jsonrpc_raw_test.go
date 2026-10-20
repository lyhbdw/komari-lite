package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	pkgflags "github.com/lyhbdw/komari-lite/agent/cmd/flags"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"github.com/lyhbdw/komari-lite/agent/version"
	"github.com/lyhbdw/komari-lite/agent/ws"
)

func TestParseV2ResponseRawResultAndErrors(t *testing.T) {
	wire := `{"jsonrpc":"2.0","id":9007199254740993,"result":{"events":[{"id":"event","method":"agent.ping","params":{"ping_task_id":9007199254740993},"created_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-01T00:01:00Z"}]}}`
	resp, err := parseV2Response([]byte(wire))
	if err != nil {
		t.Fatal(err)
	}
	id, err := json.Marshal(resp.ID)
	if err != nil || string(id) != "9007199254740993" {
		t.Errorf("response ID = %s, %v", id, err)
	}
	var result v2.EventResult
	if err := v2.BindResult(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("events = %+v", result)
	}
	var params struct {
		TaskID uint64 `json:"ping_task_id"`
	}
	if err := v2.BindParams(result.Events[0].Params, &params); err != nil || params.TaskID != 9007199254740993 {
		t.Fatalf("event params = %+v, %v", params, err)
	}
	for _, body := range []string{`{`, `{"jsonrpc":"1.0","result":{}}`, wire + ` {}`} {
		if _, err := parseV2Response([]byte(body)); err == nil {
			t.Errorf("accepted invalid response %s", body)
		}
	}
	resp, err = parseV2Response([]byte(`{"jsonrpc":"2.0","id":"bad","error":{"code":-32602,"message":"invalid params"}}`))
	if err == nil || resp == nil || resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("RPC error = %+v, %v", resp, err)
	}
	for _, resultJSON := range []string{``, `,"result":null`} {
		resp, err := parseV2Response([]byte(`{"jsonrpc":"2.0"` + resultJSON + `}`))
		if err != nil {
			t.Fatal(err)
		}
		var result v2.EventResult
		if err := v2.BindResult(resp.Result, &result); err != nil || len(result.Events) != 0 {
			t.Fatalf("absent/null result = %+v, %v", result, err)
		}
	}
}

func resetRawEventState(t *testing.T) {
	t.Helper()
	v2AckMu.Lock()
	oldAcks, oldSeen := v2AckEventIDs, v2SeenEvents
	v2AckEventIDs, v2SeenEvents = nil, make(map[string]time.Time)
	v2AckMu.Unlock()
	t.Cleanup(func() {
		v2AckMu.Lock()
		v2AckEventIDs, v2SeenEvents = oldAcks, oldSeen
		v2AckMu.Unlock()
	})
}

func TestPostV2RequestProcessesRawEvents(t *testing.T) {
	resetRawEventState(t)
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/clients/v2/rpc" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("wrong POST: %s, %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"pull","result":{"events":[{"id":"accepted","method":"agent.update","params":{"version":%q}},{"id":"invalid","method":"agent.update","params":[]}]}}`, version.Current)
	}))
	defer controller.Close()
	oldFlags := flags
	flags = &pkgflags.Config{Endpoint: controller.URL, Token: "test-token", DisableCompression: true}
	defer func() { flags = oldFlags }()
	resp, err := postV2Request(v2.NewRequest("pull", v2.MethodAgentPull, struct{}{}))
	if err != nil {
		t.Fatal(err)
	}
	processV2ResponseEvents(resp)
	processV2ResponseEvents(nil)
	processV2ResponseEvents(&v2.Response{Result: json.RawMessage(`null`)})
	processV2ResponseEvents(&v2.Response{Result: json.RawMessage(`[]`)})
	if got := snapshotV2AckEventIDs(); len(got) != 1 || got[0] != "accepted" {
		t.Fatalf("ACKs = %#v", got)
	}
}

func TestAgentWebSocketRawEventsPreserveIDs(t *testing.T) {
	resetRawEventState(t)
	upgrader := websocket.Upgrader{}
	done := make(chan struct{})
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		handleWebSocketMessages(ws.NewSafeConn(conn), done)
	}))
	defer controller.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(controller.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Current-version updates exercise real dispatch without downloading/exiting.
	messages := []string{
		`{`,
		fmt.Sprintf(`{"jsonrpc":"1.0","method":"agent.update","id":"wrong-version","params":{"version":%q}}`, version.Current),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"agent.update","id":"trailing","params":{"version":%q}} {}`, version.Current),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"agent.update","id":9007199254740993,"params":{"version":%q}}`, version.Current),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"agent.update","params":{"version":%q}}`, version.Current),
		`{"jsonrpc":"2.0","method":"agent.update","id":"bad-params","params":[]}`,
	}
	for _, message := range messages {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("agent WS handler did not exit")
	}
	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	if _, ok := v2SeenEvents["9007199254740993"]; !ok {
		t.Errorf("numeric event ID rounded: %#v", v2SeenEvents)
	}
	for _, id := range []string{"wrong-version", "trailing", ""} {
		if _, ok := v2SeenEvents[id]; ok {
			t.Errorf("unexpected seen ID %q", id)
		}
	}
}
