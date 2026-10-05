package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
	v2 "github.com/lyhbdw/komari-monitor-lite/protocol/v2"
	agent_runtime "github.com/lyhbdw/komari-monitor-lite/web/agent"
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
	if origin == "" {
		return false
	}
	if security.OriginMatchesHost(origin, r.Host) {
		return true
	}
	allowlist, _ := config.GetAs[string](config.WsAllowedOriginsKey, "")
	return security.OriginInAllowlist(origin, allowlist)
}

func GetClients(c *gin.Context) {
	if !IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	conn, err := UpgradeSafeConn(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	defer conn.Close()
	conn.GetConn().SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		var resp struct {
			Online []string             `json:"online"` // 已建立连接的客户端uuid列表
			Data   map[string]v2.Report `json:"data"`   // 最后上报的数据
		}

		resp.Online = []string{}
		resp.Data = map[string]v2.Report{}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		message := string(data)

		uuID := ""
		if message != "get" {
			if strings.HasPrefix(message, "get ") {
				uuID = strings.TrimSpace(strings.TrimPrefix(message, "get "))
			} else {
				conn.WriteJSON(gin.H{"status": "error", "error": "Invalid message"})
				continue
			}
		}

		for _, key := range agent_runtime.GetAllOnlineUUIDs() {
			if uuID != "" && key != uuID {
				continue
			}
			resp.Online = append(resp.Online, key)
		}

		for key, report := range agent_runtime.GetLatestReport() {
			if uuID != "" && key != uuID {
				continue
			}

			report.UUID = ""
			if report.CPU.Usage == 0 {
				report.CPU.Usage = 0.01
			}
			resp.Data[key] = *report
		}

		err = conn.WriteJSON(gin.H{"status": "success", "data": resp})
		if err != nil {
			return
		}
	}
}
