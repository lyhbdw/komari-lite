package public

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/api"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TwoFa    string `json:"2fa_code"`
}

const sessionCookieMaxAge = 2592000

const (
	loginWindow            = 5 * time.Minute
	loginMaxAttempts       = 8
	loginMaxBodyBytes      = 16 << 10
	loginLimiterMaxEntries = 4096
)

var loginLimiter = struct {
	sync.Mutex
	attempts map[string][]time.Time
}{attempts: make(map[string][]time.Time)}

func loginAllowed(key string, now time.Time) bool {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	cutoff := now.Add(-loginWindow)
	for existingKey, oldEntries := range loginLimiter.attempts {
		entries := oldEntries[:0]
		for _, at := range oldEntries {
			if at.After(cutoff) {
				entries = append(entries, at)
			}
		}
		if len(entries) == 0 {
			delete(loginLimiter.attempts, existingKey)
		} else {
			loginLimiter.attempts[existingKey] = entries
		}
	}
	entries := loginLimiter.attempts[key]
	if len(entries) >= loginMaxAttempts {
		loginLimiter.attempts[key] = entries
		return false
	}
	loginLimiter.attempts[key] = append(entries, now)
	if len(loginLimiter.attempts) > loginLimiterMaxEntries {
		oldestKey := ""
		var oldest time.Time
		for candidate, candidateEntries := range loginLimiter.attempts {
			if len(candidateEntries) > 0 && (oldestKey == "" || candidateEntries[len(candidateEntries)-1].Before(oldest)) {
				oldestKey, oldest = candidate, candidateEntries[len(candidateEntries)-1]
			}
		}
		if oldestKey != "" {
			delete(loginLimiter.attempts, oldestKey)
		}
	}
	return true
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "session_token",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   utils.GetScheme(c) == "https",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func Login(c *gin.Context) {
	DisablePasswordLogin, _ := config.GetAs[bool](config.DisablePasswordLoginKey, false)
	if DisablePasswordLogin {
		api.RespondError(c, http.StatusForbidden, "Password login is disabled")
		return
	}

	if !loginAllowed(c.ClientIP(), time.Now()) {
		api.RespondError(c, http.StatusTooManyRequests, "Too many login attempts")
		return
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, loginMaxBodyBytes+1))
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if len(bodyBytes) > loginMaxBodyBytes {
		api.RespondError(c, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	var data LoginRequest
	err = json.Unmarshal(bodyBytes, &data)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if data.Username == "" || data.Password == "" {
		api.RespondError(c, http.StatusBadRequest, "Invalid request body: Username and password are required")
		return
	}

	if !loginAllowed(c.ClientIP()+"\x00"+data.Username, time.Now()) {
		api.RespondError(c, http.StatusTooManyRequests, "Too many login attempts")
		return
	}

	uuid, success := accounts.CheckPassword(data.Username, data.Password)
	if !success {
		api.RespondError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	// 2FA
	user, _ := accounts.GetUserByUUID(uuid)
	if user.TwoFactor != "" { // 开启了2FA
		if data.TwoFa == "" {
			api.RespondError(c, http.StatusUnauthorized, "2FA code is required")
			return
		}
		if ok, err := accounts.Verify2Fa(uuid, data.TwoFa); err != nil || !ok {
			api.RespondError(c, http.StatusUnauthorized, "Invalid 2FA code")
			return
		}
	}
	// Create session
	session, err := accounts.CreateSession(uuid, sessionCookieMaxAge, c.Request.UserAgent(), c.ClientIP(), "password")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to create session: "+err.Error())
		return
	}
	setSessionCookie(c, session, sessionCookieMaxAge)
	auditlog.Log(c.ClientIP(), uuid, "logged in (password)", "login")
	api.RespondSuccess(c, nil)
}
func Logout(c *gin.Context) {
	// CSRF 防御：登出是状态变更操作，仅接受 POST（原 GET 可被跨站触发）。
	if c.Request.Method != http.MethodPost {
		c.Redirect(302, "/")
		return
	}
	session, _ := c.Cookie("session_token")
	accounts.DeleteSession(session)
	setSessionCookie(c, "", -1)
	auditlog.Log(c.ClientIP(), "", "logged out", "logout")
	c.Redirect(302, "/")
}
