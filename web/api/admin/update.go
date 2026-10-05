package admin

import (
	"encoding/json"
	"github.com/lyhbdw/komari-monitor-lite/database/accounts"
	"github.com/lyhbdw/komari-monitor-lite/database/auditlog"
	"github.com/lyhbdw/komari-monitor-lite/utils/geoip"
	"github.com/lyhbdw/komari-monitor-lite/web/api"
	"github.com/gin-gonic/gin"
)

// update.go
// 文件/二进制/敏感操作类的更新接口，保留为 REST handler（不走 RPC 桥）。
// 由原 web/api/admin/update/ 子包合并而来。

func UpdateUser(c *gin.Context) {
	var req struct {
		Uuid     string  `json:"uuid"`
		Name     *string `json:"username"`
		Password *string `json:"password"`
		TwoFa    string  `json:"2fa_code"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		api.RespondError(c, 400, "Invalid or missing request body: "+err.Error())
		return
	}
	if req.Uuid == "" {
		api.RespondError(c, 400, "Invalid or missing request body: uuid is required")
		return
	}
	if req.Password == nil && req.Name == nil {
		api.RespondError(c, 400, "At least one field (username or password) must be provided")
		return
	}
	if req.Name != nil && len(*req.Name) < 3 {
		api.RespondError(c, 400, "Username must be at least 3 characters long")
		return
	}
	if req.Password != nil && len(*req.Password) < 6 {
		api.RespondError(c, 400, "Password must be at least 6 characters long")
		return
	}
	if req.Password != nil {
		c.Set("2fa_code", req.TwoFa)
		if err := api.VerifySensitive2FA(c); err != nil {
			api.RespondError(c, 401, err.Error())
			return
		}
	}
	if err := accounts.UpdateUser(req.Uuid, req.Name, req.Password); err != nil {
		api.RespondError(c, 500, "Failed to update user: "+err.Error())
		return
	}
	uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), uuid.(string), "User updated", "warn")
	api.RespondSuccess(c, gin.H{"uuid": req.Uuid})
}

func UpdateMmdbGeoIP(c *gin.Context) {
	if err := geoip.UpdateDatabase(); err != nil {
		api.RespondError(c, 500, "Failed to update GeoIP database "+err.Error())
		return
	}
	uuid, _ := c.Get("uuid")
	auditlog.Log(c.ClientIP(), uuid.(string), "GeoIP database updated", "info")
	api.RespondSuccess(c, nil)
}
