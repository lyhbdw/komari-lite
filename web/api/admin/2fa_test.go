package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTwoFactorCookieUsesSecureStrictAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "https://example.test/api/admin/2fa/generate", nil)
	setTwoFactorCookie(c, "secret", 1800)
	value := strings.Join(w.Header().Values("Set-Cookie"), "\n")
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(value, want) {
			t.Fatalf("cookie %q missing %s", value, want)
		}
	}
}
