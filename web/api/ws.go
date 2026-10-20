package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/web/connection"
	"github.com/lyhbdw/komari-lite/web/security"
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
	if origin == "" {
		return false
	}
	if security.OriginMatchesHost(origin, r.Host) {
		return true
	}
	allowlist, _ := config.GetAs[string](config.WsAllowedOriginsKey, "")
	return security.OriginInAllowlist(origin, allowlist)
}
