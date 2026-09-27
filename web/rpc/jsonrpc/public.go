package jsonrpc

import (
	"context"

	"github.com/Tumb1er1376/komari-monitor-lite/database"
	"github.com/Tumb1er1376/komari-monitor-lite/database/dbcore"
	"github.com/Tumb1er1376/komari-monitor-lite/database/models"
	"github.com/Tumb1er1376/komari-monitor-lite/database/tasks"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
	"github.com/Tumb1er1376/komari-monitor-lite/utils"
)

// public.go
// 公开（guest 可访问）的只读 RPC2 方法。命名空间 public:* 对 guest 开放。
// 这些方法保持与原 REST 接口完全一致的响应形状。

func init() {
	rpc.Allow("public:*", rpc.RoleGuest)
	regPublic("getMe", publicGetMe, "Get current user info (guest-aware)")
	regPublic("getPublicSettings", publicGetPublicSettings, "Get public site settings")
	regPublic("getVersion", publicGetVersion, "Get server version")
	regPublic("getPublicPingTasks", publicGetPublicPingTasks, "List public ping tasks")
}

func regPublic(name string, h rpc.Handler, summary string) {
	RegisterWithGroupAndMeta(name, "public", h, &rpc.MethodMeta{Name: "public:" + name, Summary: summary})
}

// isLoginFromCtx 依据 meta 判断是否为已登录管理员。
func isLoginFromCtx(ctx context.Context) bool {
	if meta := rpc.MetaFromContext(ctx); meta != nil {
		return meta.Principal != nil && meta.Principal.HasRole(rpc.RoleAdmin)
	}
	return false
}

func publicGetPublicSettings(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	p, e := database.GetPublicInfo()
	if e != nil {
		return nil, rpc.MakeError(rpc.InternalError, e.Error(), nil)
	}
	return p, nil
}

func publicGetVersion(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	return map[string]any{
		"version": utils.CurrentVersion,
		"hash":    utils.VersionHash,
	}, nil
}

// publicGetMe 返回当前用户信息；未登录时返回 Guest 占位，保持原 /api/me 的扁平形状。
func publicGetMe(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	guest := map[string]any{"username": "Guest", "logged_in": false}
	meta := rpc.MetaFromContext(ctx)
	if meta == nil || meta.User == nil {
		return guest, nil
	}
	u := meta.User
	return map[string]any{
		"username":    u.Username,
		"logged_in":   true,
		"uuid":        u.UUID,
		"2fa_enabled": u.TwoFactor != "",
	}, nil
}

// hiddenClientUUIDMap 返回所有隐藏节点 uuid 的集合（查询失败时返回空集合，宁可少泄露）。
func hiddenClientUUIDMap() map[string]bool {
	hidden := map[string]bool{}
	var hiddenClients []models.Client
	db := dbcore.GetDBInstance()
	_ = db.Select("uuid").Where("hidden = ?", true).Find(&hiddenClients).Error
	for _, cli := range hiddenClients {
		hidden[cli.UUID] = true
	}
	return hidden
}

func publicGetPublicPingTasks(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	pingTasks, err := tasks.GetAllPingTasks()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}
	// 未登录（非管理员）时过滤隐藏节点的 UUID，避免通过 ping 任务暴露隐藏节点。
	isLogin := isLoginFromCtx(ctx)
	var hiddenMap map[string]bool
	if !isLogin {
		hiddenMap = hiddenClientUUIDMap()
	}
	type publicPingTask struct {
		Id        uint     `json:"id"`
		Weight    int      `json:"weight"`
		Name      string   `json:"name"`
		Clients   []string `json:"clients"`
		DefaultOn bool     `json:"default_on"`
		Type      string   `json:"type"`
		Interval  int      `json:"interval"`
	}
	out := make([]publicPingTask, len(pingTasks))
	for i, task := range pingTasks {
		clients := task.Clients
		if hiddenMap != nil {
			clients = make([]string, 0, len(task.Clients))
			for _, uuid := range task.Clients {
				if hiddenMap[uuid] {
					continue
				}
				clients = append(clients, uuid)
			}
		}
		out[i] = publicPingTask{
			Id:        task.Id,
			Weight:    task.Weight,
			Name:      task.Name,
			Clients:   clients,
			DefaultOn: task.DefaultOn,
			Type:      task.Type,
			Interval:  task.Interval,
		}
	}
	return out, nil
}
