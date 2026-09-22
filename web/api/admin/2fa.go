package admin

import (
	"image/png"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/api"
	"github.com/pquerna/otp/totp"
)

func setTwoFactorCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "2fa_secret", Value: value, Path: "/", MaxAge: maxAge, Secure: utils.GetScheme(c) == "https", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func Generate2FA(c *gin.Context) {
	secret, img, err := accounts.Generate2Fa()
	if err != nil {
		api.RespondError(c, 500, "Failed to generate 2FA: "+err.Error())
		return
	}
	setTwoFactorCookie(c, secret, 1800)
	c.Header("Content-Type", "image/png")
	c.Writer.WriteHeader(200)
	png.Encode(c.Writer, img)
}

func Enable2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	secret, _ := c.Cookie("2fa_secret")
	// CSRF 防御：code 从 POST body 读取（原走 URL query，会进浏览器历史与日志）。
	var body struct {
		Code string `json:"code"`
	}
	_ = c.ShouldBindJSON(&body)
	code := body.Code
	if secret == "" || uuid == nil || code == "" {
		api.RespondError(c, 400, "2FA secret or code not provided")
		return
	}
	if !totp.Validate(code, secret) {
		api.RespondError(c, 400, "Invalid 2FA code")
		return
	}
	err := accounts.Enable2Fa(uuid.(string), secret)
	if err != nil {
		api.RespondError(c, 500, "Failed to enable 2FA: "+err.Error())
		return
	}
	setTwoFactorCookie(c, "", -1)
	api.RespondSuccess(c, "2FA enabled successfully")
}

func Disable2FA(c *gin.Context) {
	uuid, _ := c.Get("uuid")
	err := accounts.Disable2Fa(uuid.(string))
	if err != nil {
		api.RespondError(c, 500, "Failed to disable 2FA: "+err.Error())
		return
	}
	api.RespondSuccess(c, "")
}
