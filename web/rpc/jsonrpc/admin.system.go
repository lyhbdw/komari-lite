package jsonrpc

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Tumb1er1376/komari-monitor-lite/database/dbcore"
	"github.com/Tumb1er1376/komari-monitor-lite/database/models"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/config"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/geoip"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/messageSender"
	"gorm.io/gorm"
)

// admin.system.go
// 系统/运维类 RPC2 方法（admin 命名空间）：日志和测试。

func init() {
	RegisterWithGroupAndMeta("getLogs", rpc.RoleAdmin, adminGetLogs, &rpc.MethodMeta{
		Name:    "admin:getLogs",
		Summary: "Get audit logs (paged, optionally filtered by message type)",
		Params: []rpc.ParamMeta{
			{Name: "limit", Type: "string", Description: "Page size (default 100)"},
			{Name: "page", Type: "string", Description: "One-based page number (default 1)"},
			{Name: "msg_type", Type: "string", Description: "Optional exact message type filter"},
		},
		Returns: "{ logs: Log[], total: number }",
	})
	reg("testSendMessage", adminTestSendMessage, "Send a test notification")
	reg("testGeoip", adminTestGeoip, "Test GeoIP lookup")
}

const (
	// maxAdminLogsLimit 限制单页日志条数，防止超大 limit 拖垮数据库/内存。
	maxAdminLogsLimit = 500
	// maxAdminLogsPage 防止 offset 溢出（page*limit 过大时直接拒绝）。
	maxAdminLogsPage = 100000
)

func adminGetLogs(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Limit   string `json:"limit"`
		Page    string `json:"page"`
		MsgType string `json:"msg_type"`
	}
	req.BindParams(&params)
	if params.Limit == "" {
		params.Limit = "100"
	}
	if params.Page == "" {
		params.Page = "1"
	}
	limitInt, err := strconv.Atoi(params.Limit)
	if err != nil || limitInt <= 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid limit: "+params.Limit, nil)
	}
	if limitInt > maxAdminLogsLimit {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid limit: must be <= 500", nil)
	}
	pageInt, err := strconv.Atoi(params.Page)
	if err != nil || pageInt <= 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid page: "+params.Page, nil)
	}
	if pageInt > maxAdminLogsPage {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid page: too large", nil)
	}
	db := dbcore.GetDBInstance()
	logs, total, err := queryAdminLogs(db, limitInt, pageInt, params.MsgType)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to retrieve logs: "+err.Error(), nil)
	}
	return map[string]any{"logs": logs, "total": total}, nil
}

func queryAdminLogs(db *gorm.DB, limit, page int, msgType string) ([]models.Log, int64, error) {
	var logs []models.Log
	var total int64
	offset := (page - 1) * limit
	countQuery := filterAdminLogsByMessageType(db.Model(&models.Log{}), msgType)
	logsQuery := filterAdminLogsByMessageType(db.Model(&models.Log{}), msgType)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := logsQuery.Order("time desc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func filterAdminLogsByMessageType(query *gorm.DB, msgType string) *gorm.DB {
	if msgType = strings.TrimSpace(msgType); msgType != "" {
		return query.Where("msg_type = ?", msgType)
	}
	return query
}

func adminTestSendMessage(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	// 测试消息走 SendEvent 而非 SendNotification：后者要求 Clients 携带
	// 真实节点 UUID（空 UUID 会被全部过滤并报 "none of the specified
	// clients exist"），而测试载荷只需要一个展示用的名称。
	if err := messageSender.SendEvent(models.EventMessage{
		Event:   "Test",
		Time:    time.Now().UTC(),
		Emoji:   "🔔",
		Clients: []models.Client{{Name: "Komari-Server"}},
		Message: "This is a test message from Komari.",
	}); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to send message: "+err.Error(), nil)
	}
	return nil, nil
}

func adminTestGeoip(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		IP string `json:"ip"`
	}
	req.BindParams(&params)
	ip := params.IP
	if ip == "" {
		if meta := rpc.MetaFromContext(ctx); meta != nil {
			ip = meta.RemoteIP
		}
	}
	cfg, err := config.GetAs[bool](config.GeoIpEnabledKey, false)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get configuration: "+err.Error(), nil)
	}
	if !cfg {
		return nil, rpc.MakeError(rpc.InvalidParams, "GeoIP is not enabled in the configuration.", nil)
	}
	record, err := geoip.GetGeoInfo(net.ParseIP(ip))
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to get GeoIP record: "+err.Error(), nil)
	}
	return record, nil
}
