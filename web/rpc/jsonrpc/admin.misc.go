package jsonrpc

import (
	"context"
	"strconv"

	"github.com/lyhbdw/komari-monitor-lite/database/accounts"
	"github.com/lyhbdw/komari-monitor-lite/database/auditlog"
	"github.com/lyhbdw/komari-monitor-lite/database/dbcore"
	"github.com/lyhbdw/komari-monitor-lite/database/models"
	"github.com/lyhbdw/komari-monitor-lite/database/records"
	"github.com/lyhbdw/komari-monitor-lite/database/tasks"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
	"github.com/lyhbdw/komari-monitor-lite/pkg/rpc"
)

// admin.misc.go
// 杂项 admin RPC2 方法：会话管理、设置、客户端排序。

func parseUintKey(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

func init() {
	RegisterWithGroupAndMeta("getSessions", rpc.RoleAdmin, adminGetSessions, &rpc.MethodMeta{
		Name:    "admin:getSessions",
		Summary: "List all login sessions",
		Returns: "{ current: string, data: Session[] }",
	})
	RegisterWithGroupAndMeta("deleteSession", rpc.RoleAdmin, adminDeleteSession, &rpc.MethodMeta{
		Name:    "admin:deleteSession",
		Summary: "Delete a session by token",
		Returns: "null",
	})
	rpc.MarkSensitive("admin:deleteSession")
	RegisterWithGroupAndMeta("deleteAllSessions", rpc.RoleAdmin, adminDeleteAllSessions, &rpc.MethodMeta{
		Name:    "admin:deleteAllSessions",
		Summary: "Delete all sessions",
		Returns: "null",
	})
	rpc.MarkSensitive("admin:deleteAllSessions")
	RegisterWithGroupAndMeta("getSettings", rpc.RoleAdmin, adminGetSettings, &rpc.MethodMeta{
		Name:    "admin:getSettings",
		Summary: "Get all settings",
		Returns: "object",
	})
	RegisterWithGroupAndMeta("editSettings", rpc.RoleAdmin, adminEditSettings, &rpc.MethodMeta{
		Name:    "admin:editSettings",
		Summary: "Update settings (partial)",
		Returns: "null | { restart_required: true }",
	})
	rpc.MarkSensitive("admin:editSettings")
	RegisterWithGroupAndMeta("clearAllRecords", rpc.RoleAdmin, adminClearAllRecords, &rpc.MethodMeta{
		Name:    "admin:clearAllRecords",
		Summary: "Delete all load and ping records",
		Returns: "null",
	})
	rpc.MarkSensitive("admin:clearAllRecords")
	RegisterWithGroupAndMeta("orderClients", rpc.RoleAdmin, adminOrderClients, &rpc.MethodMeta{
		Name:    "admin:orderClients",
		Summary: "Reorder clients (map of uuid->weight)",
		Returns: "null",
	})
}

func adminGetSessions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	ss, err := accounts.GetAllSessions()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to retrieve sessions: "+err.Error(), nil)
	}
	current := ""
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		current = accounts.SessionIdentifier(meta.SessionToken)
	}
	data := make([]map[string]any, 0, len(ss))
	for _, session := range ss {
		data = append(data, map[string]any{
			"id":                accounts.SessionIdentifier(session.Session),
			"uuid":              session.UUID,
			"user_agent":        session.UserAgent,
			"ip":                session.Ip,
			"login_method":      session.LoginMethod,
			"latest_online":     session.LatestOnline,
			"latest_ip":         session.LatestIp,
			"latest_user_agent": session.LatestUserAgent,
			"expires":           session.Expires,
			"created_at":        session.CreatedAt,
		})
	}
	return map[string]any{"current": current, "data": data}, nil
}

func adminDeleteSession(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		ID string `json:"id"`
	}
	req.BindParams(&params)
	if params.ID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "id is required", nil)
	}
	if err := accounts.DeleteSessionByIdentifier(params.ID); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete session: "+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete session", "info")
	return nil, nil
}

func adminDeleteAllSessions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if err := accounts.DeleteAllSessions(); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to delete all sessions: "+err.Error(), nil)
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "delete all sessions", "warn")
	return nil, nil
}

func adminGetSettings(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cst, err := config.GetAll()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get settings: "+err.Error(), nil)
	}
	return cst, nil
}

func adminEditSettings(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	cfg := make(map[string]interface{})
	if err := req.BindParams(&cfg); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing request body: "+err.Error(), nil)
	}
	removeRetiredLowResourceMode(cfg)
	enforceLiteThemeSettings(cfg)
	removeRetiredMetricStoreConfig(cfg)

	if err := config.SetMany(cfg); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to update settings: "+err.Error(), nil)
	}

	auditSettingsUpdate(ctx, cfg)
	return nil, nil
}

func auditSettingsUpdate(ctx context.Context, cfg map[string]interface{}) {
	message := "update settings: "
	for key := range cfg {
		message += key + ", "
	}
	if len(message) > 2 {
		message = message[:len(message)-2]
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, message, "info")
}

// removeRetiredLowResourceMode keeps older admin clients from recreating its
// config row after the startup migration removes it.
func removeRetiredLowResourceMode(cfg map[string]interface{}) {
	delete(cfg, "low_resource_mode")
}

func enforceLiteThemeSettings(cfg map[string]interface{}) {
	if _, ok := cfg[config.ThemeKey]; ok {
		cfg[config.ThemeKey] = "Lite"
	}
}
func removeRetiredMetricStoreConfig(cfg map[string]interface{}) {
	for _, key := range []string{
		"metric_db_driver",
		"metric_db_dsn",
		"metric_table_prefix",
		"metric_max_open_conns",
		"metric_max_idle_conns",
		"metric_rollup_minute_retention_minutes",
		"metric_rollup_five_minute_retention_minutes",
		"metric_rollup_hour_retention_hours",
		"metric_migration_target",
	} {
		delete(cfg, key)
	}
}

func adminClearAllRecords(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {

	records.DeleteAll()
	tasks.DeleteAllPingRecords()
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "clear all records", "info")
	return nil, nil
}

func adminOrderClients(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var order map[string]int
	if err := req.BindParams(&order); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid or missing request body: "+err.Error(), nil)
	}
	db := dbcore.GetDBInstance()
	for uuid, weight := range order {
		if err := db.Model(&models.Client{}).Where("uuid = ?", uuid).Update("weight", weight).Error; err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to update client weight: "+err.Error(), nil)
		}
	}
	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, "order clients", "info")
	return nil, nil
}
