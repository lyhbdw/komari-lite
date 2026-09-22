package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	v2 "github.com/komari-monitor/komari/protocol/v2"
	"github.com/komari-monitor/komari/utils/notifier"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/connection"
)

const maxAgentBodyBytes int64 = 4 << 20

func readMaybeCompressedBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return readLimited(zr, maxAgentBodyBytes)
	}
	return readLimited(r.Body, maxAgentBodyBytes)
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("request body exceeds %d bytes", limit)
	}
	return data, nil
}

func bindV2Params[T any](raw any, target *T) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

// handleV2RPC 处理一条 v2 JSON-RPC 请求。
// viaWebSocket 为 true 时表示请求来自长连接（WS），此时不刷新 POST 在线状态：
// WS 连接的在线状态由连接生命周期自行管理，POST presence 只属于 HTTP 上报者。
func handleV2RPC(uuid string, req v2.Request, allowWait, viaWebSocket bool) v2.Response {
	if req.JSONRPC != v2.Version {
		return v2.Error(req.ID, -32600, "invalid jsonrpc version", nil)
	}
	switch req.Method {
	case v2.MethodAgentReport:
		var params v2.ReportParams
		if err := bindV2Params(req.Params, &params); err != nil {
			return v2.Error(req.ID, -32602, "invalid report params", err.Error())
		}
		if err := ingestReport(uuid, params.Report, !viaWebSocket); err != nil {
			return v2.Error(req.ID, -32000, "failed to save report", err.Error())
		}
		return v2.Success(req.ID, gin.H{
			"status": "success",
			"events": agent_runtime.TakeV2Events(uuid, params.AckEventIDs, 8),
		})
	case v2.MethodAgentBasicInfo:
		var params v2.BasicInfoParams
		if err := bindV2Params(req.Params, &params); err != nil {
			return v2.Error(req.ID, -32602, "invalid basic info params", err.Error())
		}
		if err := ingestBasicInfo(uuid, params.Info, ""); err != nil {
			return v2.Error(req.ID, -32000, "failed to save basic info", err.Error())
		}
		return v2.Success(req.ID, gin.H{"status": "success"})
	case v2.MethodAgentPingResult:
		var params v2.PingResultParams
		if err := bindV2Params(req.Params, &params); err != nil {
			return v2.Error(req.ID, -32602, "invalid ping result params", err.Error())
		}
		if err := ingestPingResult(uuid, params.TaskID, params.Value); err != nil {
			return v2.Error(req.ID, -32000, "failed to save ping result", err.Error())
		}
		return v2.Success(req.ID, gin.H{"status": "success"})

	case v2.MethodAgentPull:
		var params v2.PullParams
		if err := bindV2Params(req.Params, &params); err != nil {
			return v2.Error(req.ID, -32602, "invalid pull params", err.Error())
		}
		if !viaWebSocket {
			refreshPostPresence(uuid)
		}
		agent_runtime.MarkV2Client(uuid)
		timeout := 0 * time.Second
		if allowWait {
			timeout = 25 * time.Second
		}
		return v2.Success(req.ID, gin.H{
			"events": agent_runtime.WaitV2Events(uuid, params.AckEventIDs, timeout),
		})

	default:
		return v2.Error(req.ID, -32601, "method not found", req.Method)
	}
}

func UploadV2RPC(c *gin.Context) {
	bytesBody, err := readMaybeCompressedBody(c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, v2.Error(nil, -32700, "invalid compressed body", err.Error()))
		return
	}
	var req v2.Request
	if err := json.Unmarshal(bytesBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, v2.Error(nil, -32700, "parse error", err.Error()))
		return
	}
	uuid, ok := clientUUIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, v2.Error(req.ID, -32001, "invalid token", nil))
		return
	}
	resp := handleV2RPC(uuid, req, true, false)
	status := http.StatusOK
	if resp.Error != nil {
		status = http.StatusBadRequest
	}
	c.JSON(status, resp)
}

func WebSocketV2RPC(c *gin.Context) {
	if !api.IsWebSocketUpgrade(c) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Require WebSocket upgrade"})
		return
	}
	conn, err := api.UpgradeSafeConn(c, api.EnableWebSocketCompression)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	defer conn.Close()
	conn.GetConn().SetReadLimit(maxAgentBodyBytes)

	uuid, ok := clientUUIDFromContext(c)
	if !ok {
		conn.WriteJSON(v2.Error(nil, -32001, "invalid token", nil))
		return
	}
	if oldConn, exists := agent_runtime.GetConnectedClients()[uuid]; exists {
		go oldConn.Close()
	}
	agent_runtime.SetConnectedClients(uuid, conn)
	agent_runtime.MarkV2Client(uuid)
	go notifierOnline(uuid, conn.ID)
	defer func() {
		agent_runtime.DeleteClientConditionally(uuid, conn)
		notifierOffline(uuid, conn.ID)
	}()
	if !pushQueuedV2Events(conn, uuid) {
		return
	}

	// 服务端心跳：定期发 PingMessage 控制帧。gorilla/websocket 的 agent 端
	// 默认自动回 pong，浏览器端也会自动回 pong；任何消息（含 pong）都会
	// 重置下方的读超时，因此上报间隔较长的 agent 也不会被误判为离线。
	// 写失败（含写超时）说明连接已不可用，直接退出读循环清理连接。
	heartbeat := time.NewTicker(pingPeriod)
	defer heartbeat.Stop()
	go func() {
		for range heartbeat.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(readWait))
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("client-api", "Client %s v2 connection error: %v", uuid, err)
			}
			return
		}
		message = bytes.TrimSpace(message)
		var req v2.Request
		if err := json.Unmarshal(message, &req); err != nil {
			conn.WriteJSON(v2.Error(nil, -32700, "parse error", err.Error()))
			continue
		}
		resp := handleV2RPC(uuid, req, false, true)
		if req.ID != nil {
			if err := conn.WriteJSON(resp); err != nil {
				logger.Errorf("client-api", "failed to write v2 rpc response: %v", err)
				return
			}
		}
	}
}

func pushQueuedV2Events(conn *connection.SafeConn, uuid string) bool {
	events := agent_runtime.TakeV2Events(uuid, nil, 0)
	if len(events) == 0 {
		return true
	}
	ackIDs := make([]string, 0, len(events))
	for _, event := range events {
		payload := v2.Request{JSONRPC: v2.Version, Method: event.Method, Params: event.Params}
		if err := conn.WriteJSON(payload); err != nil {
			agent_runtime.AckV2Events(uuid, ackIDs)
			logger.Errorf("client-api", "failed to push queued v2 event %s to client %s: %v", event.ID, uuid, err)
			return false
		}
		ackIDs = append(ackIDs, event.ID)
	}
	agent_runtime.AckV2Events(uuid, ackIDs)
	return true
}

func clientUUIDFromContext(c *gin.Context) (string, bool) {
	if v, ok := c.Get("client_uuid"); ok {
		if uuid, ok := v.(string); ok && uuid != "" {
			return uuid, true
		}
	}
	return "", false
}

func notifierOnline(uuid string, connID int64) {
	go func() {
		defer func() { _ = recover() }()
		notifier.OnlineNotification(uuid, connID)
	}()
}

func notifierOffline(uuid string, connID int64) {
	defer func() { _ = recover() }()
	notifier.OfflineNotification(uuid, connID)
}
