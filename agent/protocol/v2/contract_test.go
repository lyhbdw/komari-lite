package v2

// 契约测试：agent 侧协议编解码必须与 contract/ 下的共享线格式语料保持一致。
// 服务器侧有对应的契约测试（protocol/v2/contract_test.go）解析同一份语料。
// 任何一边修改 JSON 字段名/结构，对应的测试会立即失败。
// 修改协议时：先更新 contract/*.json，再更新两端实现。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// contractDir 从本测试文件位置推导仓库根下的 contract/ 目录，
// 不依赖 go test 的工作目录（包目录），IDE / go test ./... 下都成立。
func contractDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "contract")
}

func loadContract(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(contractDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract sample %s: %v", path, err)
	}
	return data
}

// rawParams 从语料请求中取出 params 的原始 JSON，用于喂给 agent 的构建器。
func rawParams(t *testing.T, sample []byte) (method string, id any, params json.RawMessage) {
	t.Helper()
	var envelope struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(sample, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return envelope.Method, envelope.ID, envelope.Params
}

func TestContractReportRequestRoundTrip(t *testing.T) {
	method, id, params := rawParams(t, loadContract(t, "report_request.json"))
	if method != MethodAgentReport {
		t.Fatalf("method = %q, want %q", method, MethodAgentReport)
	}

	var wire struct {
		Report      json.RawMessage `json:"report"`
		AckEventIDs []string        `json:"ack_event_ids"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		t.Fatalf("split report params: %v", err)
	}

	// agent 用 BuildReportRequest 构造等价请求，字段名必须与语料一致
	built := BuildReportRequest(json.RawMessage(`"req-1"`), wire.Report, wire.AckEventIDs)

	// Request.Params 经普通解码是 map；用 RawMessage 局部结构取回原始字节，
	// 与语料的 report 做逐字节语义比对（字段名/结构零漂移）。
	var builtEnvelope struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(built, &builtEnvelope); err != nil {
		t.Fatalf("decode built request: %v", err)
	}
	if builtEnvelope.Method != MethodAgentReport {
		t.Errorf("built method = %q", builtEnvelope.Method)
	}
	var builtParams struct {
		Report      json.RawMessage `json:"report"`
		AckEventIDs []string        `json:"ack_event_ids"`
	}
	if err := json.Unmarshal(builtEnvelope.Params, &builtParams); err != nil {
		t.Fatalf("decode built params: %v", err)
	}
	var builtReport, wantReport any
	if err := json.Unmarshal(builtParams.Report, &builtReport); err != nil {
		t.Fatalf("decode built report: %v", err)
	}
	if err := json.Unmarshal(wire.Report, &wantReport); err != nil {
		t.Fatalf("decode corpus report: %v", err)
	}
	if !reflect.DeepEqual(builtReport, wantReport) {
		t.Errorf("built report differs from corpus:\nbuilt: %s\nwant:  %s", builtParams.Report, wire.Report)
	}
	if !reflect.DeepEqual(builtParams.AckEventIDs, []string{"evt-1", "evt-2"}) {
		t.Errorf("built ack_event_ids = %v", builtParams.AckEventIDs)
	}
	_ = id
}

func TestContractPullResponseParsing(t *testing.T) {
	var resp Response
	if err := json.Unmarshal(loadContract(t, "pull_response.json"), &resp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	// Response.UnmarshalJSON 使用 UseNumber：超大整数 id 必须逐位保留
	idNumber, ok := resp.ID.(json.Number)
	if !ok {
		t.Fatalf("id type = %T, want json.Number", resp.ID)
	}
	if idNumber.String() != "9007199254740993" {
		t.Errorf("id = %s, precision lost", idNumber.String())
	}

	var result EventResult
	if err := BindResult(resp.Result, &result); err != nil {
		t.Fatalf("bind result: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("status = %q", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(result.Events))
	}

	// agent.ping 事件参数同样走 RawMessage + UseNumber，大整数 task id 不可丢精度
	ping := result.Events[0]
	if ping.Method != MethodAgentPing {
		t.Errorf("event 0 method = %q, want %q", ping.Method, MethodAgentPing)
	}
	var pingParams struct {
		TaskID json.Number `json:"ping_task_id"`
	}
	if err := BindParams(ping.Params, &pingParams); err != nil {
		t.Fatalf("bind ping params: %v", err)
	}
	if pingParams.TaskID.String() != "9007199254740993" {
		t.Errorf("ping_task_id = %s, precision lost", pingParams.TaskID.String())
	}

	// agent.update 事件参数按 UpdateParams 解码
	update := result.Events[1]
	if update.Method != MethodAgentUpdate {
		t.Errorf("event 1 method = %q, want %q", update.Method, MethodAgentUpdate)
	}
	var updateParams UpdateParams
	if err := BindParams(update.Params, &updateParams); err != nil {
		t.Fatalf("bind update params: %v", err)
	}
	if updateParams.Version != "1.1.14" {
		t.Errorf("update version = %q", updateParams.Version)
	}
	if updateParams.SHA256 != "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3" {
		t.Errorf("update sha256 = %q", updateParams.SHA256)
	}
}

func TestContractPingResultPayload(t *testing.T) {
	method, _, params := rawParams(t, loadContract(t, "ping_result_request.json"))
	if method != MethodAgentPingResult {
		t.Fatalf("method = %q, want %q", method, MethodAgentPingResult)
	}
	var wire struct {
		TaskID   uint   `json:"task_id"`
		PingType string `json:"ping_type"`
		Value    int    `json:"value"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		t.Fatalf("decode corpus params: %v", err)
	}
	if wire.TaskID != 42 || wire.PingType != "tcp" || wire.Value != 23 {
		t.Errorf("corpus ping params = %+v", wire)
	}
}

func TestContractMethodConstants(t *testing.T) {
	// 与服务器侧 protocol/v2 的方法名常量保持一致（wire 契约）
	want := map[string]string{
		MethodAgentReport:     "agent.report",
		MethodAgentBasicInfo:  "agent.basicInfo",
		MethodAgentPingResult: "agent.pingResult",
		MethodAgentPing:       "agent.ping",
		MethodAgentPull:       "agent.pull",
		MethodAgentUpdate:     "agent.update",
	}
	for got, expected := range want {
		if got != expected {
			t.Errorf("method constant = %q, want %q", got, expected)
		}
	}
}
