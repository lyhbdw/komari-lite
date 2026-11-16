package rpc

// 生产链路契约测试：contract/ 语料必须能穿过真实解码链
// （ParseRequests → BindParams）后保持数值精确。
// 覆盖 /api/rpc2 通用 JSON-RPC 路径（区别于 agent 专用的
// /api/clients/v2/rpc，后者有自己的 UseNumber 解码器）。

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func loadCorpus(t *testing.T, name string) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "contract", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract sample %s: %v", path, err)
	}
	return data
}

// TestCorpusBigIntSurvivesGenericRPCChain 是关键回归：
// 超过 2^53 的整数（语料中的 totalUp/totalDown）经过
// ParseRequests + BindParams 后必须逐位精确。
func TestCorpusBigIntSurvivesGenericRPCChain(t *testing.T) {
	requests, rpcErr := ParseRequests(loadCorpus(t, "report_request.json"))
	if rpcErr != nil {
		t.Fatalf("ParseRequests: %+v", rpcErr)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	req := requests[0]
	if req.Method != v2.MethodAgentReport {
		t.Fatalf("method = %q, want %q", req.Method, v2.MethodAgentReport)
	}

	var params v2.ReportParams
	if err := req.BindParams(&params); err != nil {
		t.Fatalf("BindParams: %v", err)
	}
	if params.Report.Network.TotalUp != 9007199254740993 {
		t.Errorf("totalUp = %d, want 9007199254740993 (precision lost in generic RPC chain)", params.Report.Network.TotalUp)
	}
	if params.Report.Network.TotalDown != 9007199254740992 {
		t.Errorf("totalDown = %d, want 9007199254740992", params.Report.Network.TotalDown)
	}
	if params.Report.CPU.Cores != 4 || params.Report.Ram.Total != 3221225472 {
		t.Errorf("cpu/ram = %+v/%+v", params.Report.CPU, params.Report.Ram)
	}
	if len(params.AckEventIDs) != 2 {
		t.Errorf("ack_event_ids = %v", params.AckEventIDs)
	}
}

// TestCorpusBatchParsing 验证批量请求同样走 UseNumber 解码。
func TestCorpusBatchParsing(t *testing.T) {
	ping := bytes.TrimSpace(loadCorpus(t, "ping_result_request.json"))
	basic := bytes.TrimSpace(loadCorpus(t, "basic_info_request.json"))
	batch := append(append(append([]byte("["), ping...), ','), basic...)
	batch = append(batch, ']')

	requests, rpcErr := ParseRequests(batch)
	if rpcErr != nil {
		t.Fatalf("ParseRequests: %+v", rpcErr)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if requests[0].Method != v2.MethodAgentPingResult || requests[1].Method != v2.MethodAgentBasicInfo {
		t.Fatalf("methods = %q, %q", requests[0].Method, requests[1].Method)
	}
	var params v2.PingResultParams
	if err := requests[0].BindParams(&params); err != nil {
		t.Fatalf("BindParams: %v", err)
	}
	if params.TaskID != 42 || params.Value != 23 {
		t.Errorf("ping params = %+v", params)
	}
}

// TestCorpusNumericIDPreserved 验证请求 id 的数字精度同样受保护。
func TestCorpusNumericIDPreserved(t *testing.T) {
	// 语料 pull_response.json 中的 9007199254740993 同样出现在请求 id 位置时
	// 必须保持 json.Number 精确值（构造等价请求，避免响应格式缺少 method）。
	inline := []byte(`{"jsonrpc":"2.0","method":"agent.pull","params":{},"id":9007199254740993}`)
	requests, rpcErr := ParseRequests(inline)
	if rpcErr != nil {
		t.Fatalf("ParseRequests: %+v", rpcErr)
	}
	id, ok := requests[0].ID.(json.Number)
	if !ok {
		t.Fatalf("id type = %T, want json.Number", requests[0].ID)
	}
	if id.String() != "9007199254740993" {
		t.Errorf("id = %s, precision lost", id.String())
	}
}
