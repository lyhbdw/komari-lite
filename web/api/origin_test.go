package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestWebSocketOriginBypassRemoved 守卫：GIN_MODE/KOMARI_WS_DISABLE_ORIGIN 组合
// 不得再提供 Origin 校验旁路（生产安全不应依赖运行模式环境变量）。
func TestWebSocketOriginBypassRemoved(t *testing.T) {
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

	for _, mode := range []string{"release", "", "debug", "test"} {
		t.Setenv("GIN_MODE", mode)
		t.Setenv("KOMARI_ENV", "development")
		if checkWebSocketOriginForContext(nil, r) {
			t.Fatalf("origin bypass must not exist in mode %q", mode)
		}
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
