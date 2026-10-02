package api

import (
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Tumb1er1376/komari-monitor-lite/database/accounts"
	"github.com/Tumb1er1376/komari-monitor-lite/database/clients"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
	"github.com/Tumb1er1376/komari-monitor-lite/utils/log"
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

func extractClientToken(c *gin.Context) string {
	return ExtractClientTokenFromRequest(c.Request)
}

// legacyQueryTokenWarnMu 节流 URL query token 的兼容警告，
// 避免高频上报的 agent 刷爆日志。
var legacyQueryTokenWarnMu sync.Mutex
var legacyQueryTokenWarnLast time.Time

func warnLegacyQueryToken(r *http.Request) {
	legacyQueryTokenWarnMu.Lock()
	defer legacyQueryTokenWarnMu.Unlock()
	if time.Since(legacyQueryTokenWarnLast) < time.Hour {
		return
	}
	legacyQueryTokenWarnLast = time.Now()
	logger.Warnf("auth", "client token passed via URL query on %s is legacy compatibility and will be removed; upgrade the agent to use the Authorization header", r.URL.Path)
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
			warnLegacyQueryToken(r)
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
