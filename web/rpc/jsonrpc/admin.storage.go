package jsonrpc

import (
	"context"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/pkg/rpc"
)

// admin.storage.go
// 存储维护类 admin RPC2 方法。
//
// 背景：metricstore.ReclaimSpace（VACUUM / OPTIMIZE）与 InspectStorage
// 此前实现了完整的独占门与前后体积测量，但没有任何入口调用（admin RPC、
// 定时任务、CLI 都没有），属于"设计了但没接线"。SQLite 的删除只把页挂到
// freelist，文件不收缩；metrics.db 的空洞会随保留清理持续累积。
//
//   - admin:inspectStorage：只读查询当前存储体积，无破坏性；
//   - admin:reclaimSpace：执行独占维护（VACUUM），标记为敏感操作（需 2FA）。
//     执行期间写入与 compaction 会被操作门阻塞，属于预期行为。

func init() {
	RegisterWithGroupAndMeta("inspectStorage", rpc.RoleAdmin, adminInspectStorage, &rpc.MethodMeta{
		Name:    "admin:inspectStorage",
		Summary: "Inspect metric store physical storage (driver, maintenance action, size)",
		Returns: "{ driver: string, action: string, size_bytes: number }",
	})
	RegisterWithGroupAndMeta("reclaimSpace", rpc.RoleAdmin, adminReclaimSpace, &rpc.MethodMeta{
		Name:    "admin:reclaimSpace",
		Summary: "Run metric store space reclamation (VACUUM/OPTIMIZE). Blocks writes while running",
		Returns: "{ driver: string, action: string, before_bytes: number, after_bytes: number }",
	})
	rpc.MarkSensitive("admin:reclaimSpace")
}

func adminInspectStorage(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	info, err := metricstore.InspectStorage(ctx)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to inspect metric storage: "+err.Error(), nil)
	}
	return map[string]any{
		"driver":     string(info.Driver),
		"action":     string(info.Action),
		"size_bytes": info.Size,
	}, nil
}

func adminReclaimSpace(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	result, err := metricstore.ReclaimSpace(ctx)
	actor, ip := auditActor(ctx)
	if err != nil {
		auditlog.Log(ip, actor, "reclaim metric store space (failed: "+err.Error()+")", "warn")
		return nil, rpc.MakeError(rpc.InternalError, "Failed to reclaim metric store space: "+err.Error(), nil)
	}
	auditlog.Log(ip, actor, "reclaim metric store space", "warn")
	resp := map[string]any{
		"driver":       string(result.Driver),
		"action":       string(result.Action),
		"before_bytes": result.Before,
		"after_bytes":  result.After,
	}
	if result.BeforeSizeError != nil {
		resp["before_size_error"] = result.BeforeSizeError.Error()
	}
	if result.AfterSizeError != nil {
		resp["after_size_error"] = result.AfterSizeError.Error()
	}
	return resp, nil
}
