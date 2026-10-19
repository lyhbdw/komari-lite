package security

import "github.com/gin-gonic/gin"

// TwoFACodeParamKeys defines the accepted parameter aliases for one-time codes.
var TwoFACodeParamKeys = []string{"2fa_code", "two_factor_code", "otp"}

// ExtractTwoFACodeFromParams extracts a code from an object-shaped RPC payload.
func ExtractTwoFACodeFromParams(params map[string]any) string {
	for _, key := range TwoFACodeParamKeys {
		if value, ok := params[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// ExtractTwoFACodeFromHeaderOrQuery extracts a 2FA code without reading the body.
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
	for _, key := range TwoFACodeParamKeys {
		if code := c.Query(key); code != "" {
			return code
		}
	}
	return ""
}
