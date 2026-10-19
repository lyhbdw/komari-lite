package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTwoFACodeKeysAreConsistentAcrossQueryAndRPCParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, key := range TwoFACodeParamKeys {
		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+key+"=123456", nil)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req
			if got := ExtractTwoFACodeFromHeaderOrQuery(ctx); got != "123456" {
				t.Fatalf("query code = %q, want 123456", got)
			}
			if got := ExtractTwoFACodeFromParams(map[string]any{key: "654321"}); got != "654321" {
				t.Fatalf("parameter code = %q, want 654321", got)
			}
		})
	}
}

func TestExtractTwoFACodeFromParamsIgnoresInvalidValues(t *testing.T) {
	if got := ExtractTwoFACodeFromParams(map[string]any{"otp": 123456}); got != "" {
		t.Fatalf("non-string code = %q, want empty", got)
	}
	if got := ExtractTwoFACodeFromParams(map[string]any{"otp": ""}); got != "" {
		t.Fatalf("empty code = %q, want empty", got)
	}
}
