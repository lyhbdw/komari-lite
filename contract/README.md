# Agent ↔ Server Wire Contract

`protocol/v2`（服务器）与 `agent/protocol/v2`（agent）是两个独立 Go module 中
各自维护的协议实现：服务器侧持有完整类型模型，agent 侧用 RawMessage + UseNumber
做惰性解码（更小的二进制、与服务器发布节奏解耦）。这种分工是有意的，代价是
wire 格式（JSON 字段名、方法名、结构）存在两处定义。

本目录是 **wire 格式的唯一事实来源**：

- `*.json` — 协议语料样本。每种消息一份，字段尽量全覆盖。
- `protocol/v2/contract_test.go` — 服务器侧测试：语料必须能无损解码进服务器类型。
- `agent/protocol/v2/contract_test.go` — agent 侧测试：语料必须与 agent 的
  构建器/解析器行为一致（含大整数精度）。
- `pkg/rpc/corpus_contract_test.go` — 通用 `/api/rpc2` 链路：语料穿过
  `ParseRequests → BindParams` 后数值精确（UseNumber 回归）。
- `web/api/client/corpus_contract_test.go` — agent 专用 `/api/clients/v2/rpc`
  链路：语料穿过 `inboundV2Request → bindV2Params` 后数值精确。

## 修改协议时

1. 先更新本目录的语料（新字段就同时加进两端相关样本）；
2. 更新 `protocol/v2/jsonrpc.go` 和/或 `agent/protocol/v2/jsonrpc.go`；
3. 跑 `go test ./protocol/...` 和 `cd agent && go test ./protocol/...`——
   任何一端与语料不符，对应测试立即失败，不会带病上线。

## 已知取舍

- 服务器的 `Request.ID` 是 `any`（float64 语义），agent 的
  `Response.UnmarshalJSON` 用 `UseNumber`（json.Number 逐位保留）。
  语料 `pull_response.json` 中的超大 id 同时覆盖两种行为，精度断言在 agent 侧。
- 方法名常量在两端各自定义，由各自的 `TestContractMethodConstants` 钉死。
