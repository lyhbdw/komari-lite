package jsonrpc

import (
	"context"

	"github.com/Tumb1er1376/komari-monitor-lite/database/auditlog"
	"github.com/Tumb1er1376/komari-monitor-lite/database/clients"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
	v2 "github.com/Tumb1er1376/komari-monitor-lite/protocol/v2"
	"github.com/Tumb1er1376/komari-monitor-lite/web/public"
	agent_runtime "github.com/Tumb1er1376/komari-monitor-lite/web/agent"
)

// admin.agent.go
// Agent 自动升级的 admin RPC2 方法。
//
// admin:upgradeAgents 向指定（或全部）节点下发 agent.update 事件：
//   - 在线节点：事件已在队列，下一次 report/pull 即取走；WS 长连接由
//     report 响应携带（agent 每 interval 秒上报一次，延迟最多一个上报周期）。
//   - 离线节点：事件保留 30 分钟 TTL，节点恢复上线后通过 pull 补领。
//
// agent 收到事件后从本面板 /download/agent/<version>/ 下载二进制并自替换。
// 标记为敏感操作（需 2FA）。

func init() {
	RegisterWithGroupAndMeta("upgradeAgents", rpc.RoleAdmin, adminUpgradeAgents, &rpc.MethodMeta{
		Name:    "admin:upgradeAgents",
		Summary: "Broadcast an agent.update event so agents self-upgrade from panel-hosted assets",
		Returns: "{ version: string, dispatched: int }",
	})
	rpc.MarkSensitive("admin:upgradeAgents")

	RegisterWithGroupAndMeta("getAgentAssetVersion", rpc.RoleAdmin, adminGetAgentAssetVersion, &rpc.MethodMeta{
		Name:    "admin:getAgentAssetVersion",
		Summary: "Report the agent version currently served by this panel",
		Returns: "{ version: string }",
	})
}

func adminGetAgentAssetVersion(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return map[string]any{"version": public.AgentAssetVersion}, nil
}

func adminUpgradeAgents(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Version string   `json:"version"`
		UUIDs   []string `json:"uuids"`
	}
	req.BindParams(&params)

	// 未指定版本时使用面板当前托管版本。
	version := params.Version
	if version == "" {
		version = public.AgentAssetVersion
	}

	// 目标节点集合：空表示全部。
	targets := params.UUIDs
	if len(targets) == 0 {
		all, err := clients.GetAllClientBasicInfo()
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to list clients: "+err.Error(), nil)
		}
		targets = make([]string, 0, len(all))
		for _, c := range all {
			targets = append(targets, c.UUID)
		}
	}
	if len(targets) == 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "No clients to upgrade", nil)
	}

	updateParams := v2.UpdateParams{Version: version}
	dispatched := 0
	for _, uuid := range targets {
		agent_runtime.DispatchV2Update(uuid, updateParams)
		dispatched++
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "upgrade agents to "+version, "warn")

	return map[string]any{
		"version":    version,
		"dispatched": dispatched,
	}, nil
}
