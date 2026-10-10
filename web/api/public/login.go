package public

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/database/accounts"
	"github.com/lyhbdw/komari-lite/database/auditlog"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/utils"
	"github.com/lyhbdw/komari-lite/utils/attemptlimit"
	"github.com/lyhbdw/komari-lite/web/api"

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
	return attemptlimit.Allow(loginLimiter.attempts, key, now, loginWindow, loginMaxAttempts, loginLimiterMaxEntries)
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
