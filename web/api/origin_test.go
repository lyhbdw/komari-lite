package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWebSocketOriginBypassRequiresDevelopmentMode(t *testing.T) {
	t.Setenv("KOMARI_WS_DISABLE_ORIGIN", "true")
	r := httptest.NewRequest("GET", "http://api.example/socket", nil)
	r.Host = "api.example"
	r.Header.Set("Origin", "https://evil.example")

	db, err := gorm.Open(sqlite.Open("file:origin_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := config.SetDb(db); err != nil {
		t.Fatalf("set config db: %v", err)
	}

	for _, mode := range []string{"release", ""} {
		t.Setenv("GIN_MODE", mode)
		if CheckWebSocketOrigin(r) {
			t.Fatalf("origin bypass enabled in mode %q", mode)
		}
	}
	t.Setenv("GIN_MODE", "debug")
	if !CheckWebSocketOrigin(r) {
		t.Fatal("explicit development origin bypass was not honored")
	}
}

func TestAuthenticatedAgentMayUseOriginlessV2WebSocket(t *testing.T) {
	req := httptest.NewRequest("GET", "http://api.example/api/clients/v2/rpc", nil)
	req.Host = "api.example"
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req
	ctx.Set("role", RoleClient)

	if !checkWebSocketOriginForContext(ctx, req) {
		t.Fatal("authenticated originless agent WebSocket should be accepted")
	}
}

func TestOriginlessNonAgentWebSocketRemainsRejected(t *testing.T) {
	tests := []struct {
		name string
		path string
		role string
	}{
		{name: "guest agent endpoint", path: "/api/clients/v2/rpc", role: RoleGuest},
		{name: "client non-agent endpoint", path: "/api/rpc2", role: RoleClient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://api.example"+tt.path, nil)
			req.Host = "api.example"
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req
			ctx.Set("role", tt.role)
			if checkWebSocketOriginForContext(ctx, req) {
				t.Fatal("originless WebSocket should remain rejected")
			}
		})
	}
}
