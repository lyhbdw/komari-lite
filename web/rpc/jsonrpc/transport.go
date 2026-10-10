package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-lite/database/accounts"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
	"github.com/lyhbdw/komari-lite/web/api"
	"github.com/lyhbdw/komari-lite/web/security"
)

const (
	maxJSONRPCBodyBytes = 1 << 20
	// wsIdleTimeout: 一条消息到达后允许的最大等待时长，超时视为空闲并关闭连接。
	wsIdleTimeout = 5 * time.Minute
	// wsSessionRecheckInterval: 每处理 N 条消息重校验一次会话有效性。
	wsSessionRecheckEvery = 32
)

// OnRpcRequest 是 /api/rpc2 的统一入口：GET 升级为 WebSocket，POST 处理单条/批量 JSON-RPC。
func OnRpcRequest(c *gin.Context) {
	// GET -> WebSocket
	if c.Request.Method == http.MethodGet {
		serveWebSocket(c)
		return
	}

	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		return
	}
	servePost(c)
}

// CallFromGin 供传统 gin handler / 路由桥转调 RPC 方法。
// 复用 IdentityMiddleware 已识别的 principal；未识别时兜底调用 IdentifyPrincipal。
func CallFromGin(c *gin.Context, method string, params any) *rpc.JsonRpcResponse {
	meta := buildContextMeta(c)
	req := &rpc.JsonRpcRequest{Version: rpc.RPC_VERSION, Method: method, Params: params}
	return dispatchWithSensitive(c.Request.Context(), c, meta, req)
}

// dispatchWithSensitive 在统一分发前对敏感方法补充二次验证，使各调用入口行为一致。
// 对已通过命名空间权限校验的敏感方法，要求调用方满足敏感操作 2FA。
// 校验基于当前用户 Principal，Dispatch 仍为权威鉴权点。
//
// 2FA code 按"每请求"提取:优先取自本条 RPC 请求的 params(2fa_code/two_factor_code/otp),
// 这对 WebSocket 长连接尤其重要——每条敏感消息携带新鲜的 TOTP 码,避免连接级握手码过期或被复用;
// 缺失时回退到 X-2FA-Code / X-Two-Factor-Code 请求头与 query(REST/直连场景)。
//
// 若请求已被 RequireSensitive2FA 中间件校验过(sensitive_2fa_verified),则跳过,避免重复校验。
func dispatchWithSensitive(ctx context.Context, c *gin.Context, meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) *rpc.JsonRpcResponse {
	if meta != nil && meta.Principal != nil && (c == nil || !c.GetBool("sensitive_2fa_verified")) &&
		rpc.IsSensitive(req.Method) && rpc.CheckPrincipal(meta.Principal, req.Method) {
		code := extractRequestTwoFACode(req)
		if code == "" && c != nil {
			code = security.ExtractTwoFACodeFromHeaderOrQuery(c)
		}
		if err := api.VerifySensitive2FACore(meta.Principal.UserUUID, code); err != nil {
			return rpc.ErrorResponse(req.ID, rpc.PermissionDenied, err.Error(), nil)
		}
	}
	return Dispatch(ctx, meta, req)
}

// extractRequestTwoFACode 从单条 RPC 请求的命名参数中提取 2FA code。
// 仅支持对象(map)形式的 params;按 2fa_code / two_factor_code / otp 顺序查找。
func extractRequestTwoFACode(req *rpc.JsonRpcRequest) string {
	for _, key := range []string{"2fa_code", "two_factor_code", "otp"} {
		if v, ok := rpc.GetParamAs[string](req, key); ok && v != "" {
			return v
		}
	}
	return ""
}

// wsSessionStillValid 重校验建立连接时的主体身份是否仍然有效。
// 会话被删除（如登出、管理员删除会话）后长连接不应继续以该身份执行方法。
func wsSessionStillValid(meta *rpc.ContextMeta) bool {
	if meta == nil || meta.Principal == nil || meta.Principal.Type != rpc.PrincipalUser {
		return true // 匿名/agent 连接无会话可失效
	}
	if meta.SessionToken == "" {
		// 无会话 token（如 API Key 场景），无法重校验，保持原状。
		return true
	}
	_, err := accounts.GetSession(meta.SessionToken)
	return err == nil
}

func serveWebSocket(c *gin.Context) {
	conn, err := api.UpgradeSafeConn(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "Failed to upgrade to WebSocket." + err.Error()})
		return
	}
	defer conn.Close()
	rawConn := conn.GetConn()
	rawConn.SetReadLimit(maxJSONRPCBodyBytes)

	// 服务端定期发 ping 控制帧，配合读超时检测半开/死连接。
	var pingMu sync.Mutex // 由 ping ticker 串行写入控制帧
	stopPinger := make(chan struct{})
	pingTicker := time.NewTicker(wsIdleTimeout / 2)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-stopPinger:
				return
			case <-pingTicker.C:
				pingMu.Lock()
				_ = conn.WriteMessage(websocket.PingMessage, []byte("keepalive"))
				pingMu.Unlock()
			}
		}
	}()
	defer close(stopPinger)

	// pong（及任何帧）都会刷新读超时。
	_ = rawConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
	rawConn.SetPongHandler(func(string) error {
		return rawConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
	})

	meta := buildContextMeta(c)
	messages := 0
	for {
		var req rpc.JsonRpcRequest
		if err := conn.ReadJSON(&req); err != nil {
			var se *json.SyntaxError
			var ute *json.UnmarshalTypeError
			if errors.As(err, &se) || errors.As(err, &ute) {
				conn.WriteJSON(rpc.ErrorResponse(nil, rpc.InvalidRequest, "bad request: "+err.Error(), nil))
				continue
			}
			// 其它视为连接/IO 错误，结束循环
			break
		}
		messages++
		_ = rawConn.SetReadDeadline(time.Now().Add(wsIdleTimeout))
		if jerr := req.Validate(); jerr != nil {
			conn.WriteJSON(jerr.ResponseWithID(req.ID))
			continue
		}
		// 周期性重校验会话：会话失效（登出/删除）即断开长连接。
		if messages%wsSessionRecheckEvery == 0 && !wsSessionStillValid(meta) {
			conn.WriteJSON(rpc.ErrorResponse(req.ID, rpc.PermissionDenied, "session is no longer valid", nil))
			break
		}
		// 同步写：SafeConn 内部有锁，串行写避免响应乱序与并发竞态。
		conn.WriteJSON(dispatchWithSensitive(context.Background(), c, meta, &req))
	}
}

func servePost(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxJSONRPCBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, rpc.ErrorResponse(nil, rpc.ParseError, "read body error", err.Error()))
		return
	}
	if len(body) > maxJSONRPCBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, rpc.ErrorResponse(nil, rpc.ParseError, "request body too large", nil))
		return
	}
	requests, jerr := rpc.ParseRequests(body)
	if jerr != nil {
		c.JSON(http.StatusBadRequest, jerr.Response())
		return
	}
	meta := buildContextMeta(c)

	responses := make([]*rpc.JsonRpcResponse, 0, len(requests))
	for _, rreq := range requests {
		responses = append(responses, dispatchWithSensitive(c.Request.Context(), c, meta, rreq))
	}
	// 单条直接对象，批量数组（符合 JSON-RPC 2.0）。
	if len(responses) == 1 {
		c.JSON(http.StatusOK, responses[0])
	} else {
		c.JSON(http.StatusOK, responses)
	}
}

// buildContextMeta 从 gin.Context 构建 *rpc.ContextMeta。
// 复用 IdentityMiddleware 已识别的 principal(api.GetPrincipal)；若未识别则兜底调用
// api.IdentifyPrincipal。填充 principal、Permission(兼容)、User、各 UUID、token 等字段。
func buildContextMeta(c *gin.Context) *rpc.ContextMeta {
	// 优先读取中间件已识别的 principal；未识别时兜底自行识别(如 /api/rpc2 请求)。
	p := api.GetPrincipal(c)
	if p == nil {
		p = api.IdentifyPrincipal(c)
	}

	meta := &rpc.ContextMeta{
		Principal:  p,
		Permission: p.PrimaryRole(), // 兼容现有 handler 与 Dispatch
		RemoteIP:   c.ClientIP(),
		UserAgent:  c.GetHeader("User-Agent"),
	}

	// 根据主体类型填充具体字段。
	switch p.Type {
	case rpc.PrincipalUser:
		meta.UserUUID = p.UserUUID
		if session, err := c.Cookie("session_token"); err == nil && session != "" {
			meta.SessionToken = session
			if user, err := accounts.GetUserBySession(session); err == nil {
				meta.User = &user
			}
		}
	case rpc.PrincipalAgent:
		meta.ClientUUID = p.ClientUUID
		meta.ClientToken = api.ExtractClientTokenFromRequest(c.Request)
	}

	return meta
}
