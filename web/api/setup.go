package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/accounts"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
)

const (
	setupMaxBodyBytes = 16 << 10
	setupMinPassword  = 8
)

var setupLimiter = struct {
	sync.Mutex
	attempts map[string][]time.Time
}{attempts: make(map[string][]time.Time)}

func setupAllowed(key string, now time.Time) bool {
	setupLimiter.Lock()
	defer setupLimiter.Unlock()
	cutoff := now.Add(-5 * time.Minute)
	if entries := setupLimiter.attempts[key]; len(entries) >= 8 {
		setupLimiter.attempts[key] = entries
		return false
	}
	entries := setupLimiter.attempts[key]
	for _, at := range entries {
		if at.After(cutoff) {
			entries = append(entries, at)
		}
	}
	entries = append(entries, now)
	setupLimiter.attempts[key] = entries
	return true
}

// accountCount returns the number of existing accounts.
func accountCount() (int64, error) {
	var count int64
	err := dbcore.GetDBInstance().Model(&models.User{}).Count(&count).Error
	return count, err
}

// SetupStatus reports whether first-run setup is available.
// The setup endpoints are open only while the database has no accounts.
func SetupStatus(c *gin.Context) {
	count, err := accountCount()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to check accounts: "+err.Error())
		return
	}
	RespondSuccess(c, gin.H{"setup_required": count == 0})
}

// SetupCreateAdmin creates the first administrator account.
// It refuses once any account exists, so the endpoint self-closes
// after the first successful setup.
func SetupCreateAdmin(c *gin.Context) {
	if !setupAllowed(c.ClientIP(), time.Now()) {
		RespondError(c, http.StatusTooManyRequests, "Too many attempts")
		return
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, setupMaxBodyBytes+1))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if len(bodyBytes) > setupMaxBodyBytes {
		RespondError(c, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}
	if data.Username == "" || data.Password == "" {
		RespondError(c, http.StatusBadRequest, "Username and password are required")
		return
	}
	if len(data.Password) < setupMinPassword {
		RespondError(c, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	count, err := accountCount()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to check accounts: "+err.Error())
		return
	}
	if count > 0 {
		RespondError(c, http.StatusForbidden, "Setup is already completed")
		return
	}
	if _, err := accounts.CreateAccount(data.Username, data.Password); err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to create account: "+err.Error())
		return
	}
	RespondSuccess(c, nil)
}
