package jsonrpc

import (
	"context"

	"github.com/lyhbdw/komari-lite/database/auditlog"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/internal/migrations"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

// admin.migration.go
// legacy monitoring 迁移的 admin RPC2 方法。
//
// 背景：dbcore 启动时不再为 records / records_long_term / gpu_records /
// ping_records 旧表建表或写入，历史数据统一走 metric store。若升级时旧表
// 仍存在，管理员可通过这两个方法显式检查并执行导入+清理（即 dbcore 注释
// 中承诺的"升级向导"的服务端入口）：
//   - admin:getLegacyMigrationStatus：只读检查，无破坏性；
//   - admin:runLegacyMigration：导入旧表数据到 metric store，随后 drop
//     旧表并写完成 marker。标记为敏感操作（需 2FA）。

func init() {
	RegisterWithGroupAndMeta("getLegacyMigrationStatus", rpc.RoleAdmin, adminGetLegacyMigrationStatus, &rpc.MethodMeta{
		Name:    "admin:getLegacyMigrationStatus",
		Summary: "Check whether legacy monitoring tables need migration (read-only)",
		Returns: "{ required: bool, summary: LegacyMonitoringSummary }",
	})
	RegisterWithGroupAndMeta("runLegacyMigration", rpc.RoleAdmin, adminRunLegacyMigration, &rpc.MethodMeta{
		Name:    "admin:runLegacyMigration",
		Summary: "Import legacy monitoring tables into the metric store, then drop them (destructive)",
		Returns: "{ stats: { records: int, gpu: int, ping: int } }",
	})
	rpc.MarkSensitive("admin:runLegacyMigration")
}

func adminGetLegacyMigrationStatus(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	required, summary, err := migrations.LegacyMonitoringMigrationRequired(dbcore.GetDBInstance())
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to inspect legacy monitoring data: "+err.Error(), nil)
	}
	return map[string]any{
		"required": required,
		"summary":  summary,
	}, nil
}

func adminRunLegacyMigration(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	db := dbcore.GetDBInstance()
	required, _, err := migrations.LegacyMonitoringMigrationRequired(db)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to inspect legacy monitoring data: "+err.Error(), nil)
	}
	if !required {
		return nil, rpc.MakeError(rpc.InvalidParams, "Legacy monitoring migration is not required (already migrated or no legacy data)", nil)
	}
	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}

	// 导入旧表数据到 metric store（错误全部向上返回，不吞）。
	stats, err := migrations.MigrateLegacyMonitoring(ctx, db, store, nil)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to migrate legacy monitoring data: "+err.Error(), nil)
	}

	// 导入成功后 drop 旧表并写完成 marker（marker 最后写，失败不会
	// 被误报为已完成迁移）。
	if err := migrations.CompleteLegacyMonitoringMigration(db, nil); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to finalize legacy monitoring migration: "+err.Error(), nil)
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "run legacy monitoring migration", "warn")
	return map[string]any{
		"stats": map[string]any{
			"records": stats.Records,
			"gpu":     stats.GPU,
			"ping":    stats.Ping,
		},
	}, nil
}
