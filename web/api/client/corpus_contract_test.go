package client

// agent 专用上报路径（/api/clients/v2/rpc）的契约测试：
// contract/ 语料经 inboundV2Request（UseNumber）+ bindV2Params 后
// 必须保持数值精确。与 pkg/rpc 的通用 /api/rpc2 链路覆盖互补。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	v2 "github.com/lyhbdw/komari-lite/protocol/v2"
)

func loadReportCorpus(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "contract", "report_request.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract sample %s: %v", path, err)
	}
	return data
}

// TestCorpusAgentReportDecode 覆盖 agent.report 的真实解码链。
func TestCorpusAgentReportDecode(t *testing.T) {
	var req inboundV2Request
	if err := json.Unmarshal(loadReportCorpus(t), &req); err != nil {
		t.Fatalf("decode inbound v2 request: %v", err)
	}
	if req.Method != v2.MethodAgentReport {
		t.Fatalf("method = %q, want %q", req.Method, v2.MethodAgentReport)
	}

	var params v2.ReportParams
	if err := bindV2Params(req.Params, &params); err != nil {
		t.Fatalf("bindV2Params: %v", err)
	}
	if params.Report.Network.TotalUp != 9007199254740993 {
		t.Errorf("totalUp = %d, want 9007199254740993 (precision lost on agent path)", params.Report.Network.TotalUp)
	}
	if params.Report.Network.TotalDown != 9007199254740992 {
		t.Errorf("totalDown = %d, want 9007199254740992", params.Report.Network.TotalDown)
	}
	if params.Report.CPU.Cores != 4 || params.Report.CPU.Usage != 12.5 {
		t.Errorf("cpu = %+v", params.Report.CPU)
	}
	if params.Report.GPU == nil || params.Report.GPU.DetailedInfo[0].Temperature != 55 {
		t.Errorf("gpu = %+v", params.Report.GPU)
	}
	if len(params.AckEventIDs) != 2 || params.AckEventIDs[0] != "evt-1" {
		t.Errorf("ack_event_ids = %v", params.AckEventIDs)
	}
}
