package jsonrpc

import (
	"context"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"

	"gorm.io/gorm/clause"
)

// admin.notification.go
// 通知相关 RPC2 方法（admin 命名空间）：离线通知。

func init() {
	// offline notifications
	reg("listOfflineNotifications", adminListOfflineNotifications, "List offline notifications")
	reg("editOfflineNotification", adminEditOfflineNotification, "Edit offline notifications")
	reg("enableOfflineNotification", adminEnableOfflineNotification, "Enable offline notifications for clients")
	reg("disableOfflineNotification", adminDisableOfflineNotification, "Disable offline notifications for clients")
}

// reg 是 admin 命名空间方法的注册便捷封装。
func reg(name string, h rpc.Handler, summary string) {
	RegisterWithGroupAndMeta(name, rpc.RoleAdmin, h, &rpc.MethodMeta{Name: "admin:" + name, Summary: summary})
}

func adminListOfflineNotifications(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var notifications []models.OfflineNotification
	if err := dbcore.GetDBInstance().Model(&models.OfflineNotification{}).Find(&notifications).Error; err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to list offline notifications: "+err.Error(), nil)
	}
	return notifications, nil
}

func adminEditOfflineNotification(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var notifications []models.OfflineNotification
	if err := req.BindParams(&notifications); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}
	if len(notifications) == 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "At least one notification is required", nil)
	}
	for _, noti := range notifications {
		if noti.Client == "" {
			return nil, rpc.MakeError(rpc.InvalidParams, "Client UUID cannot be empty", nil)
		}
		if noti.GracePeriod <= 0 {
			return nil, rpc.MakeError(rpc.InvalidParams, "GracePeriod must be a positive integer", nil)
		}
		if noti.Cooldown < 0 {
			return nil, rpc.MakeError(rpc.InvalidParams, "Cooldown must be a non-negative integer", nil)
		}
	}
	err := dbcore.GetDBInstance().Model(&models.OfflineNotification{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client"}},
			DoUpdates: clause.AssignmentColumns([]string{"enable", "grace_period", "cooldown"}),
		}).
		Select("*").Create(notifications).Error
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to edit offline notifications: "+err.Error(), nil)
	}
	return nil, nil
}

// setOfflineNotificationEnable 是 enable/disable 的共享实现。
func setOfflineNotificationEnable(req *rpc.JsonRpcRequest, enable bool) *rpc.JsonRpcError {
	var uuids []string
	if err := req.BindParams(&uuids); err != nil {
		return rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}
	notifications := make([]models.OfflineNotification, 0, len(uuids))
	for _, uuid := range uuids {
		notifications = append(notifications, models.OfflineNotification{Client: uuid, Enable: enable})
	}
	err := dbcore.GetDBInstance().Model(&models.OfflineNotification{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client"}},
			DoUpdates: clause.AssignmentColumns([]string{"enable"}),
		}).
		Select("client", "enable").Create(notifications).Error
	if err != nil {
		return rpc.MakeError(rpc.InternalError, "Failed to update offline notifications: "+err.Error(), nil)
	}
	return nil
}

func adminEnableOfflineNotification(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if e := setOfflineNotificationEnable(req, true); e != nil {
		return nil, e
	}
	return nil, nil
}

func adminDisableOfflineNotification(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	if e := setOfflineNotificationEnable(req, false); e != nil {
		return nil, e
	}
	return nil, nil
}
