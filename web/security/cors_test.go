package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCorsMiddlewareValidatesAPIOrigins(t *testing.T) {
	setupCORSConfigDB(t, "")
	router := setupCORSRouter("https://allowed.example")

	tests := []struct {
		name            string
		origin          string
		wantStatus      int
		wantAllowOrigin string
	}{
		{
			name:       "allows API requests without Origin",
			wantStatus: http.StatusOK,
		},
		{
			name:            "allows same host Origin",
			origin:          "https://api.example",
			wantStatus:      http.StatusOK,
			wantAllowOrigin: "https://api.example",
		},
		{
			name:            "allows configured Origin",
			origin:          "https://allowed.example",
			wantStatus:      http.StatusOK,
			wantAllowOrigin: "https://allowed.example",
		},
		{
			name:       "rejects unlisted Origin",
			origin:     "https://evil.example",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performCORSRequest(router, http.MethodGet, "/api/ping", "api.example", tt.origin)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantAllowOrigin)
			}
		})
	}
}

func TestCorsMiddlewareHandlesAPIPreflight(t *testing.T) {
	setupCORSConfigDB(t, "")
	router := setupCORSRouter("https://allowed.example")

	allowed := performCORSRequest(router, http.MethodOptions, "/api/ping", "api.example", "https://allowed.example")
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight status = %d, want %d", allowed.Code, http.StatusNoContent)
	}
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.example" {
		t.Fatalf("allowed preflight Access-Control-Allow-Origin = %q", got)
	}

	rejected := performCORSRequest(router, http.MethodOptions, "/api/ping", "api.example", "https://evil.example")
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("rejected preflight status = %d, want %d", rejected.Code, http.StatusForbidden)
	}
}

func TestCorsMiddlewareRejectsAuthorizationPreflightFromUnknownOrigin(t *testing.T) {
	setupCORSConfigDB(t, "")
	router := setupCORSRouter("")

	request := httptest.NewRequest(http.MethodOptions, "/api/ping", nil)
	request.Host = "api.example"
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Access-Control-Request-Headers", "authorization")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestCorsMiddlewareSkipsNonAPIPaths(t *testing.T) {
	setupCORSConfigDB(t, "")
	router := setupCORSRouter("")

	response := performCORSRequest(router, http.MethodGet, "/public", "api.example", "https://evil.example")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCorsMiddlewareRejectsWildcardAllowlistWithCredentials(t *testing.T) {
	setupCORSConfigDB(t, "")
	router := setupCORSRouter("*")
	response := performCORSRequest(router, http.MethodGet, "/api/ping", "api.example", "https://evil.example")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("wildcard allow origin = %q", got)
	}
}

func setupCORSRouter(allowlist string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewCorsController(allowlist).Middleware())
	router.GET("/api/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/public", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return router
}

func setupCORSConfigDB(t *testing.T, _ string) {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite test db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := config.SetDb(db); err != nil {
		t.Fatalf("set config db: %v", err)
	}
}

func performCORSRequest(handler http.Handler, method, path, host, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}
