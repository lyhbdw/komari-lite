package v2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBindParamsPreservesTypedInputs(t *testing.T) {
	for _, raw := range []interface{}{[]byte("abc"), json.RawMessage(`"YWJj"`)} {
		var got string
		if err := BindParams(raw, &got); err != nil || got != "YWJj" {
			t.Errorf("BindParams(%T) = %q, %v; want base64 string YWJj", raw, got, err)
		}
	}
	var got struct {
		TaskID uint64 `json:"task_id"`
	}
	for _, raw := range []interface{}{map[string]interface{}{"task_id": uint64(9007199254740993)}, struct {
		TaskID uint64 `json:"task_id"`
	}{9007199254740993}} {
		if err := BindParams(raw, &got); err != nil || got.TaskID != 9007199254740993 {
			t.Fatalf("typed input %T: got %+v, %v", raw, got, err)
		}
	}
	if err := BindParams(make(chan int), &got); err == nil {
		t.Fatal("unmarshalable typed params unexpectedly accepted")
	}
}

func TestBindRawParamsMissingNullAndInvalid(t *testing.T) {
	for _, raw := range []interface{}{nil, json.RawMessage(nil), json.RawMessage{}, json.RawMessage(`null`)} {
		got := struct {
			Value int `json:"value"`
		}{7}
		if err := BindParams(raw, &got); err != nil || got.Value != 7 {
			t.Errorf("null-like %T(%v): got %+v, %v", raw, raw, got, err)
		}
	}
	for _, raw := range []string{` `, `{`, `[]`, `true`, `"text"`, `{"value":"bad"}`, `{"value":1} {}`} {
		var got struct {
			Value int `json:"value"`
		}
		if err := BindParams(json.RawMessage(raw), &got); err == nil {
			t.Errorf("invalid params %s accepted", raw)
		}
	}
}

func TestResponseEventRawMessageRoundTrip(t *testing.T) {
	wire := ` {"jsonrpc":"2.0","id":"pull","result":{"status":"success","events":[{"id":"event-1","method":"agent.ping","params":{"ping_task_id":9007199254740993},"created_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-01T00:01:00Z"}]}} `
	var resp Response
	if err := json.Unmarshal([]byte(wire), &resp); err != nil {
		t.Fatal(err)
	}
	var result EventResult
	if err := BindResult(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].ID != "event-1" || result.Events[0].Method != MethodAgentPing || result.Status != "success" {
		t.Fatalf("wrong event envelope: %+v", result)
	}
	var params struct {
		TaskID uint64 `json:"ping_task_id"`
	}
	if err := BindParams(result.Events[0].Params, &params); err != nil || params.TaskID != 9007199254740993 {
		t.Fatalf("lost event integer: %+v, %v", params, err)
	}
	for _, value := range []interface{}{resp, result.Events[0]} {
		b, err := json.Marshal(value)
		if err != nil || !strings.Contains(string(b), `"ping_task_id":9007199254740993`) {
			t.Errorf("raw JSON not preserved: %s, %v", b, err)
		}
	}
}

func TestOutgoingRequestsKeepTypedParamsAndNotifications(t *testing.T) {
	params := struct {
		Count uint64 `json:"count"`
	}{9007199254740993}
	request, err := json.Marshal(Request{JSONRPC: Version, Method: MethodAgentPull, Params: params, ID: "typed"})
	if err != nil {
		t.Fatal(err)
	}
	if string(request) != `{"jsonrpc":"2.0","method":"agent.pull","params":{"count":9007199254740993},"id":"typed"}` {
		t.Fatalf("typed request: %s", request)
	}
	for _, payload := range [][]byte{NewNotification(MethodAgentPull, params), BuildReportPayload([]byte(`{"uptime":9007199254740993}`))} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(payload, &fields); err != nil {
			t.Fatal(err)
		}
		if _, exists := fields["id"]; exists {
			t.Fatalf("notification has id: %s", payload)
		}
	}
	ping, err := json.Marshal(BuildPingResultPayload(42, "tcp", -1, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil || string(ping) != `{"jsonrpc":"2.0","method":"agent.pingResult","params":{"finished_at":"2026-01-01T00:00:00Z","ping_type":"tcp","task_id":42,"value":-1}}` {
		t.Fatalf("ping wire: %s, %v", ping, err)
	}
}
