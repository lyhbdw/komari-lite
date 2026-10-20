package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func newV2RPCTestRouter(uuid string) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("client_uuid", uuid) })
	router.POST("/rpc", UploadV2RPC)
	router.GET("/rpc", WebSocketV2RPC)
	return router
}

func TestBindV2ParamsPreservesTypedAndRawContracts(t *testing.T) {
	for _, raw := range []any{[]byte("abc"), json.RawMessage(`"YWJj"`)} {
		var got string
		if err := bindV2Params(raw, &got); err != nil || got != "YWJj" {
			t.Errorf("bindV2Params(%T) = %q, %v; want base64 string YWJj", raw, got, err)
		}
	}
	for _, raw := range []any{nil, json.RawMessage(nil), json.RawMessage{}, json.RawMessage(`null`)} {
		got := v2.PullParams{AckEventIDs: []string{"keep"}}
		if err := bindV2Params(raw, &got); err != nil || len(got.AckEventIDs) != 1 {
			t.Errorf("null-like %T(%v): got %+v, %v", raw, raw, got, err)
		}
	}
	for _, raw := range []any{v2.PingResultParams{TaskID: 9007199254740993}, map[string]any{"task_id": uint64(9007199254740993)}, json.RawMessage(`{"task_id":9007199254740993}`)} {
		var got v2.PingResultParams
		if err := bindV2Params(raw, &got); err != nil || got.TaskID != 9007199254740993 {
			t.Fatalf("typed/raw params %T: %+v, %v", raw, got, err)
		}
	}
	for _, raw := range []any{make(chan int), json.RawMessage(` `), json.RawMessage(`{`), json.RawMessage(`[]`), json.RawMessage(`true`), json.RawMessage(`"text"`), json.RawMessage(`{"task_id":"bad"}`)} {
		var got v2.PingResultParams
		if err := bindV2Params(raw, &got); err == nil {
			t.Errorf("invalid params %T(%v) accepted", raw, raw)
		}
	}
}

func BenchmarkV2InboundReportDecode(b *testing.B) {
	body := []byte(`{"jsonrpc":"2.0","method":"agent.report","id":"report","params":{"report":{"cpu":{"usage":12.5},"ram":{"total":8589934592,"used":2147483648},"disk":{"total":100000000000,"used":20000000000},"network":{"up":123,"down":456,"totalUp":100000000,"totalDown":200000000},"uptime":12345,"process":42},"ack_event_ids":["event-1"]}}`)
	b.Run("raw", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var req inboundV2Request
			var params v2.ReportParams
			if err := json.Unmarshal(body, &req); err != nil {
				b.Fatal(err)
			}
			if err := bindV2Params(req.Params, &params); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy-any", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var req v2.Request
			var params v2.ReportParams
			if err := json.Unmarshal(body, &req); err != nil {
				b.Fatal(err)
			}
			if err := bindV2Params(req.Params, &params); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestUploadV2RPCPreservesNumericID(t *testing.T) {
	router := newV2RPCTestRouter("unused")
	for _, id := range []string{`9007199254740993`, `0`, `"request"`, `null`, ``} {
		body := `{"jsonrpc":"2.0","method":"unknown"`
		if id != "" {
			body += `,"id":` + id
		}
		body += `}`
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		want := id
		if id == `null` {
			want = ""
		}
		if string(fields["id"]) != want {
			t.Errorf("request id %q echoed as %s, want %s", id, fields["id"], want)
		}
		var rpcErr struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(fields["error"], &rpcErr); err != nil || rpcErr.Code != -32601 {
			t.Fatalf("error = %s, %v", fields["error"], err)
		}
	}
}
