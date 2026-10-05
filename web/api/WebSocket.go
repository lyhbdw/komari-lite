package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
	"github.com/lyhbdw/komari-monitor-lite/web/connection"
	"github.com/lyhbdw/komari-monitor-lite/web/security"
)

type WebSocketUpgradeOption func(*websocket.Upgrader)

func IsWebSocketUpgrade(c *gin.Context) bool {
	return websocket.IsWebSocketUpgrade(c.Request)
}

func EnableWebSocketCompression(upgrader *websocket.Upgrader) {
	upgrader.EnableCompression = true
}

func UpgradeWebSocket(c *gin.Context, options ...WebSocketUpgradeOption) (*websocket.Conn, error) {
	if !IsWebSocketUpgrade(c) {
		return nil, fmt.Errorf("require websocket upgrade")
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return checkWebSocketOriginForContext(c, r)
		},
	}
	for _, option := range options {
		option(&upgrader)
	}
	return upgrader.Upgrade(c.Writer, c.Request, nil)
}

// UpgradeSafeConn upgrades the request to a WebSocket and wraps it with the
// synchronized connection used by the Agent and JSON-RPC transports.
func UpgradeSafeConn(c *gin.Context, options ...WebSocketUpgradeOption) (*connection.SafeConn, error) {
	unsafeConn, err := UpgradeWebSocket(c, options...)
	if err != nil {
		return nil, err
	}
	return connection.NewSafeConn(unsafeConn), nil
}

func checkWebSocketOriginForContext(c *gin.Context, r *http.Request) bool {
	if c != nil && r.URL.Path == "/api/clients/v2/rpc" && GetRole(c) == RoleClient && r.Header.Get("Origin") == "" {
		return true
	}
	origin := r.Header.Get("Origin")
	// 不提供基于 GIN_MODE 的 Origin 校验旁路：生产安全不应依赖运行模式环境变量。
	// 测试/本地联调请通过配置项（WsAllowedOriginsKey / 关闭 WsOriginCheckEnabledKey）放行。
	enabled, err := config.GetAs[bool](config.WsOriginCheckEnabledKey, true)
	if err != nil {
		enabled = true
	}
	if !enabled {
		return true
	}
	if origin == "" {
		return false
	}
	if security.OriginMatchesHost(origin, r.Host) {
		return true
	}
	allowlist, _ := config.GetAs[string](config.WsAllowedOriginsKey, "")
	return security.OriginInAllowlist(origin, allowlist)
}
