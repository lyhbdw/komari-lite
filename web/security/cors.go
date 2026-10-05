package security

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
)

// CorsController 保存 CORS 中间件的可热更新状态。
// API CORS 来源校验为系统强制安全特性，始终处于启用状态。
type CorsController struct {
	mu             sync.RWMutex
	allowedOrigins string
}

// NewCorsController 使用初始配置构造一个 CORS 控制器。
func NewCorsController(allowedOrigins string) *CorsController {
	return &CorsController{
		allowedOrigins: allowedOrigins,
	}
}

// Update 根据配置事件刷新 CORS 相关状态。返回是否有字段发生变化。
func (ctrl *CorsController) Update(event config.ConfigEvent) bool {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	changed := false
	if ok, t := config.IsChangedT[string](event, config.CorsAllowedOriginsKey); ok {
		ctrl.allowedOrigins = t
		changed = true
	}
	return changed
}

func (ctrl *CorsController) snapshot() string {
	ctrl.mu.RLock()
	defer ctrl.mu.RUnlock()
	return ctrl.allowedOrigins
}

// Middleware 返回执行严格 CORS 校验的 gin 中间件。
func (ctrl *CorsController) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAPIRequestPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		corsAllowedOrigins := ctrl.snapshot()
		origin := c.GetHeader("Origin")
		allowOrigin := ""
		if origin != "" && (OriginMatchesHost(origin, c.Request.Host) ||
			OriginInAllowlist(origin, corsAllowedOrigins)) {
			allowOrigin = origin
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Length, Content-Type, Authorization, Accept, X-CSRF-Token, X-Requested-With, Set-Cookie, X-2FA-Code, X-Two-Factor-Code")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Authorization, Set-Cookie")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "43200") // 12 hours
		}

		if c.Request.Method == http.MethodOptions {
			if allowOrigin != "" {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		if origin != "" && allowOrigin == "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func isAPIRequestPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}
