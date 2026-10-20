package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func TestV2RPCParamsAndWireErrors(t *testing.T) {
	setupRawV2Ingestion(t)
	// Missing/null params have always behaved like a zero-value struct, not
	// invalid params. In particular an empty report is accepted by validation.
	cases := []struct {
		name, body, id string
		code           int
	}{
		{"missing", `{"jsonrpc":"2.0","method":"agent.report","id":"missing"}`, `"missing"`, 0},
		{"null", `{"jsonrpc":"2.0","method":"agent.report","params":null,"id":"null"}`, `"null"`, 0},
		{"empty", `{"jsonrpc":"2.0","method":"agent.report","params":{},"id":"empty"}`, `"empty"`, 0},
		{"array", `{"jsonrpc":"2.0","method":"agent.report","params":[],"id":"array"}`, `"array"`, -32602},
		{"string", `{"jsonrpc":"2.0","method":"agent.report","params":"bad","id":"string"}`, `"string"`, -32602},
		{"boolean", `{"jsonrpc":"2.0","method":"agent.report","params":true,"id":"boolean"}`, `"boolean"`, -32602},
		{"wrong-field", `{"jsonrpc":"2.0","method":"agent.report","params":{"report":{"ram":{"total":"bad"}}},"id":"field"}`, `"field"`, -32602},
		{"fractional-int", `{"jsonrpc":"2.0","method":"agent.pingResult","params":{"task_id":1.5},"id":"fraction"}`, `"fraction"`, -32602},
		{"version", `{"jsonrpc":"1.0","method":"agent.report","id":"version"}`, `"version"`, -32600},
		{"malformed", `{"jsonrpc":"2.0","params":`, ``, -32700},
		{"trailing", `{"jsonrpc":"2.0","method":"agent.report"} {}`, ``, -32700},
		{"notification", `{"jsonrpc":"2.0","method":"agent.report"}`, ``, 0},
		{"null-id", `{"jsonrpc":"2.0","method":"agent.report","id":null}`, ``, 0},
		{"invalid-notification", `{"jsonrpc":"2.0","method":"agent.report","params":[]}`, ``, -32602},
	}
	for _, transport := range []string{"http", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			uuid := "v2-errors-" + transport
			cleanupRawV2Client(t, uuid)
			router := newV2RPCTestRouter(uuid)
			var conn *websocket.Conn
			if transport == "websocket" {
				controller := httptest.NewServer(router)
				defer controller.Close()
				var err error
				conn, _, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(controller.URL, "http")+"/rpc", http.Header{"Origin": []string{controller.URL}})
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					var body []byte
					if conn != nil {
						if err := conn.WriteMessage(websocket.TextMessage, []byte(tc.body)); err != nil {
							t.Fatal(err)
						}
						// A following request acts as a barrier: notifications (including
						// null IDs and invalid params) must not emit a response first.
						if tc.id == "" && tc.code != -32700 {
							if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0","method":"barrier","id":"barrier"}`)); err != nil {
								t.Fatal(err)
							}
						}
						conn.SetReadDeadline(time.Now().Add(3 * time.Second))
						_, message, err := conn.ReadMessage()
						if err != nil {
							t.Fatal(err)
						}
						body = message
					} else {
						rec := httptest.NewRecorder()
						router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(tc.body)))
						wantStatus := http.StatusOK
						if tc.code != 0 {
							wantStatus = http.StatusBadRequest
						}
						if rec.Code != wantStatus {
							t.Fatalf("HTTP %d, want %d: %s", rec.Code, wantStatus, rec.Body.String())
						}
						body = rec.Body.Bytes()
					}
					var resp struct {
						JSONRPC string          `json:"jsonrpc"`
						ID      json.RawMessage `json:"id"`
						Result  json.RawMessage `json:"result"`
						Error   *v2.RPCError    `json:"error"`
					}
					if err := json.Unmarshal(body, &resp); err != nil {
						t.Fatal(err)
					}
					wantID, wantCode := tc.id, tc.code
					if conn != nil && tc.id == "" && tc.code != -32700 {
						wantID, wantCode = `"barrier"`, -32601
					}
					if resp.JSONRPC != "2.0" || string(resp.ID) != wantID {
						t.Fatalf("wrong response envelope: %s", body)
					}
					if wantCode == 0 {
						if resp.Error != nil || !strings.Contains(string(resp.Result), `"status":"success"`) {
							t.Fatalf("wrong result: %s", body)
						}
					} else if resp.Error == nil || resp.Error.Code != wantCode || len(resp.Result) != 0 {
						t.Fatalf("wrong error (want %d): %s", wantCode, body)
					}
				})
			}
		})
	}
}
