package v2

// 契约测试：服务器侧协议类型必须与 contract/ 下的共享线格式语料保持一致。
// agent 侧有对应的契约测试（agent/protocol/v2/contract_test.go）解析同一份语料。
// 任何一边修改 JSON 字段名/结构，对应的测试会立即失败。
// 修改协议时：先更新 contract/*.json，再更新两端实现。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// contractDir 从本测试文件位置推导仓库根下的 contract/ 目录，
// 不依赖 go test 的工作目录（包目录），IDE / go test ./... 下都成立。
func contractDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "contract")
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

func decodeParams(t *testing.T, params any, target any) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("re-marshal params: %v", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode params into %T: %v", target, err)
	}
}

// rawSampleParams 从语料请求中取出 params 对象的原始 JSON 字节，
// 直接解码进目标类型——不经过 any 中转，避免 float64 往返干扰精度断言。
func rawSampleParams(t *testing.T, name string) json.RawMessage {
	t.Helper()
	var envelope struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(loadContract(t, name), &envelope); err != nil {
		t.Fatalf("split params from %s: %v", name, err)
	}
	return envelope.Params
}

func TestContractReportRequest(t *testing.T) {
	var req Request
	if err := json.Unmarshal(loadContract(t, "report_request.json"), &req); err != nil {
		t.Fatalf("decode report request: %v", err)
	}
	if req.JSONRPC != Version {
		t.Errorf("jsonrpc = %q, want %q", req.JSONRPC, Version)
	}
	if req.Method != MethodAgentReport {
		t.Errorf("method = %q, want %q", req.Method, MethodAgentReport)
	}

	var params ReportParams
	if err := json.Unmarshal(rawSampleParams(t, "report_request.json"), &params); err != nil {
		t.Fatalf("decode params into ReportParams: %v", err)
	}

	r := params.Report
	if r.UUID != "9f634fd9-3180-45cc-82b3-11b19d6d3780" {
		t.Errorf("uuid = %q", r.UUID)
	}
	if r.CPU.Name != "AMD EPYC 9554" || r.CPU.Cores != 4 || r.CPU.Arch != "x86_64" || r.CPU.Usage != 12.5 {
		t.Errorf("cpu = %+v", r.CPU)
	}
	if r.Ram.Total != 3221225472 || r.Ram.Used != 1610612736 {
		t.Errorf("ram = %+v", r.Ram)
	}
	if r.Swap.Total != 1073741824 || r.Swap.Used != 0 {
		t.Errorf("swap = %+v", r.Swap)
	}
	if r.Load.Load1 != 0.15 || r.Load.Load5 != 0.22 || r.Load.Load15 != 0.31 {
		t.Errorf("load = %+v", r.Load)
	}
	if r.Disk.Total != 21474836480 || r.Disk.Used != 8589934592 {
		t.Errorf("disk = %+v", r.Disk)
	}
	// totalUp/totalDown 超过 float53，int64 字段必须精确落位
	if r.Network.Up != 1024 || r.Network.Down != 2048 {
		t.Errorf("network up/down = %+v", r.Network)
	}
	if r.Network.TotalUp != 9007199254740993 || r.Network.TotalDown != 9007199254740992 {
		t.Errorf("network totals = %+v (precision lost?)", r.Network)
	}
	if r.Connections.TCP != 120 || r.Connections.UDP != 45 {
		t.Errorf("connections = %+v", r.Connections)
	}
	if r.GPU == nil || r.GPU.Count != 1 || r.GPU.AverageUsage != 33.3 || len(r.GPU.DetailedInfo) != 1 {
		t.Fatalf("gpu = %+v", r.GPU)
	}
	gpu := r.GPU.DetailedInfo[0]
	if gpu.Name != "NVIDIA RTX 4090" || gpu.MemoryTotal != 24564 || gpu.MemoryUsed != 1024 || gpu.Utilization != 33.3 || gpu.Temperature != 55 {
		t.Errorf("gpu device = %+v", gpu)
	}
	if r.Uptime != 864000 || r.Process != 180 {
		t.Errorf("uptime/process = %d/%d", r.Uptime, r.Process)
	}
	if !r.UpdatedAt.Equal(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("updated_at = %s", r.UpdatedAt)
	}
	if !reflect.DeepEqual(params.AckEventIDs, []string{"evt-1", "evt-2"}) {
		t.Errorf("ack_event_ids = %v", params.AckEventIDs)
	}

	// 往返：重新编码后再解码，结构必须稳定
	roundTrip, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var again ReportParams
	if err := json.Unmarshal(roundTrip, &again); err != nil {
		t.Fatalf("decode round-trip: %v", err)
	}
	if !reflect.DeepEqual(params, again) {
		t.Errorf("round-trip mismatch:\n first: %+v\nsecond: %+v", params, again)
	}
}

func TestContractPingResultRequest(t *testing.T) {
	var req Request
	if err := json.Unmarshal(loadContract(t, "ping_result_request.json"), &req); err != nil {
		t.Fatalf("decode ping result request: %v", err)
	}
	if req.Method != MethodAgentPingResult {
		t.Errorf("method = %q, want %q", req.Method, MethodAgentPingResult)
	}
	var params PingResultParams
	decodeParams(t, req.Params, &params)
	if params.TaskID != 42 || params.PingType != "tcp" || params.Value != 23 {
		t.Errorf("params = %+v", params)
	}
	if !params.FinishedAt.Equal(time.Date(2026, 11, 16, 12, 0, 0, 123456789, time.UTC)) {
		t.Errorf("finished_at = %s", params.FinishedAt)
	}
}

func TestContractBasicInfoRequest(t *testing.T) {
	var req Request
	if err := json.Unmarshal(loadContract(t, "basic_info_request.json"), &req); err != nil {
		t.Fatalf("decode basic info request: %v", err)
	}
	if req.Method != MethodAgentBasicInfo {
		t.Errorf("method = %q, want %q", req.Method, MethodAgentBasicInfo)
	}
	var params BasicInfoParams
	decodeParams(t, req.Params, &params)
	if params.Info["os"] != "Debian GNU/Linux 13 (trixie)" {
		t.Errorf("info os = %v", params.Info["os"])
	}
	if params.Info["ipv6"] != "::cafe::2" {
		t.Errorf("info ipv6 = %v", params.Info["ipv6"])
	}
}

func TestContractPullResponse(t *testing.T) {
	var resp Response
	if err := json.Unmarshal(loadContract(t, "pull_response.json"), &resp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.ID == nil {
		t.Fatal("missing id")
	}
	var result struct {
		Status string `json:"status"`
		Events []Event `json:"events"`
	}
	decodeParams(t, resp.Result, &result)
	if result.Status != "success" {
		t.Errorf("status = %q", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(result.Events))
	}
	ping := result.Events[0]
	if ping.Method != MethodAgentPing {
		t.Errorf("event 0 method = %q, want %q", ping.Method, MethodAgentPing)
	}
	update := result.Events[1]
	if update.Method != MethodAgentUpdate {
		t.Errorf("event 1 method = %q, want %q", update.Method, MethodAgentUpdate)
	}
	var updateParams UpdateParams
	raw, err := json.Marshal(update.Params)
	if err != nil {
		t.Fatalf("marshal event params: %v", err)
	}
	if err := json.Unmarshal(raw, &updateParams); err != nil {
		t.Fatalf("decode update params: %v", err)
	}
	if updateParams.Version != "1.1.14" || updateParams.SHA256 != "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3" {
		t.Errorf("update params = %+v", updateParams)
	}
	if !update.CreatedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("created_at = %s", update.CreatedAt)
	}
}

func TestContractErrorResponse(t *testing.T) {
	var resp Response
	if err := json.Unmarshal(loadContract(t, "error_response.json"), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("missing error")
	}
	if resp.Error.Code != -32000 || resp.Error.Message != "failed to save report" || resp.Error.Data != "disk full" {
		t.Errorf("error = %+v", resp.Error)
	}
}

func TestContractMethodConstants(t *testing.T) {
	// 方法名是 wire 契约的一部分：与 contract/ 语料中出现的字符串一一对应
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
