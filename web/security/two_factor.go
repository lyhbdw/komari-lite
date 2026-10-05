package security

import "github.com/gin-gonic/gin"

// ExtractTwoFACodeFromHeaderOrQuery 从请求头或 Query 参数提取 2FA 验证码（不读取 Body，避免消费请求体）。
func ExtractTwoFACodeFromHeaderOrQuery(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if code := c.GetHeader("X-2FA-Code"); code != "" {
		return code
	}
	if code := c.GetHeader("X-Two-Factor-Code"); code != "" {
		return code
	}
	for _, key := range []string{"2fa_code", "two_factor_code", "otp"} {
		if code := c.Query(key); code != "" {
			return code
		}
	}
	return ""
}
