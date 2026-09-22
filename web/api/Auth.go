package api

import (
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/gorm"
)

const (
	RoleAdmin  = "admin"
	RoleClient = "client"
	RoleGuest  = "guest"
)

// sessionUpdateThrottle 节流同一会话的 UpdateLatest 调用：
// 每个会话在 updateLatestThrottleInterval 内只写一次库，避免每个请求
// 都触发一次 accounts UPDATE。
var sessionUpdateThrottle = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

const updateLatestThrottleInterval = 5 * time.Minute

// shouldUpdateLatest 返回该会话是否应当更新 latest_online（节流后）。
func shouldUpdateLatest(session string) bool {
	now := time.Now()
	sessionUpdateThrottle.Lock()
	defer sessionUpdateThrottle.Unlock()
	if last, ok := sessionUpdateThrottle.last[session]; ok && now.Sub(last) < updateLatestThrottleInterval {
		return false
	}
	// 简单容量控制：条目过多时整体重置。
	if len(sessionUpdateThrottle.last) > 10000 {
		sessionUpdateThrottle.last = map[string]time.Time{}
	}
	sessionUpdateThrottle.last[session] = now
	return true
}

// IdentityMiddleware 统一身份识别中间件，在路由栈最外层运行。
// 负责识别当前请求者身份（Admin / Client / Guest），并写入 Context。
// 身份识别统一委托给 IdentifyPrincipal，同时保留现有 handler 使用的
// role/uuid/session/client_uuid 字段。
func IdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := IdentifyPrincipal(c)
		SetPrincipal(c, p)

		// 写入兼容字段。
		c.Set("role", p.PrimaryRole())
		switch p.Type {
		case rpc.PrincipalUser:
			if session, err := c.Cookie("session_token"); err == nil && session != "" {
				c.Set("session", session)
				if shouldUpdateLatest(session) {
					accounts.UpdateLatest(session, c.Request.UserAgent(), c.ClientIP())
				}
			}
			c.Set("uuid", p.UserUUID)
		case rpc.PrincipalAgent:
			c.Set("client_uuid", p.ClientUUID)
		}

		c.Next()
	}
}

// RequireRole 声明式权限校验中间件，仅允许指定角色通过。
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := GetRole(c)
		for _, role := range allowedRoles {
			if current == role {
				c.Next()
				return
			}
		}
		RespondError(c, http.StatusUnauthorized, "Unauthorized.")
		c.Abort()
	}
}

// GetRole 获取当前请求的角色
func GetRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		return RoleGuest
	}
	if s, ok := role.(string); ok {
		return s
	}
	return RoleGuest
}

// --- 私有站点访问控制 ---

var publicPaths = []string{
	"/ping",
	"/api/public",
	"/api/login",
	"/api/me",

	"/api/version",
	"/api/recent",
	"/api/admin",    // 由 RequireRole 处理
	"/api/clients/", // 由 RequireRole 处理
	"/api/preview/", // 预览令牌校验后放行
}

// PrivateSiteMiddleware 私有站点访问控制。
// 依赖 IdentityMiddleware 已设置的 role，对未认证的访客在私有站点模式下进行拦截。
func PrivateSiteMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 已认证用户直接放行
		if GetRole(c) != RoleGuest {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// 公开路径直接放行
		for _, p := range publicPaths {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		// 非 API 路径直接放行（静态资源等）
		if !strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		// 非私有站点直接放行
		privateSite, err := config.GetAs[bool](config.PrivateSiteKey, false)
		if err != nil {
			RespondError(c, http.StatusInternalServerError, "Failed to get configuration.")
			c.Abort()
			return
		}
		if !privateSite {
			c.Next()
			return
		}

		RespondError(c, http.StatusUnauthorized, "Private site is enabled, please login first.")
		c.Abort()
	}
}

func extractClientToken(c *gin.Context) string {
	return ExtractClientTokenFromRequest(c.Request)
}

func ExtractClientTokenFromRequest(r *http.Request) string {
	const prefix = "Bearer "
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authorization, prefix) {
		token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
		if token != "" && !strings.ContainsAny(token, " 	\r\n") {
			return token
		}
	}

	// Transitional compatibility for agents installed before Lite migration.
	// Keep the legacy query-token path limited to the agent v2 transport only.
	if strings.HasPrefix(r.URL.Path, "/api/clients/v2/rpc") {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token != "" && !strings.ContainsAny(token, " 	\r\n") {
			return token
		}
	}
	return ""
}

func checkTokenAndGetUUID(token string) (string, error) {
	uuid, err := clients.GetClientUUIDByToken(token)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return uuid, nil
}
