package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lyhbdw/komari-lite/web/api"
)

func liteTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", c.GetHeader("X-Test-Role"))
		c.Next()
	})
	Register(r)
	return r
}

func assertDisabledResponse(t *testing.T, status int, header http.Header, body []byte) {
	t.Helper()
	if status != http.StatusNotFound || !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
		t.Fatalf("disabled route: status=%d type=%q body=%q", status, header.Get("Content-Type"), body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["status"] != "error" || got["message"] != "Not found" {
		t.Fatalf("disabled route returned data instead of 404: %s", body)
	}
}

func TestLegacyClientRoutesReturn404(t *testing.T) {
	r := liteTestRouter()
	for _, path := range []string{"/api/clients", "/api/clients/v1/rpc"} {
		for _, role := range []string{api.RoleGuest, api.RoleClient, api.RoleAdmin} {
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead, http.MethodConnect, http.MethodTrace} {
				t.Run(path+"/"+role+"/"+method, func(t *testing.T) {
					req := httptest.NewRequest(method, path, nil)
					req.Header.Set("X-Test-Role", role)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					assertDisabledResponse(t, w.Code, w.Header(), w.Body.Bytes())
				})
			}
		}
	}
}

func TestLegacyClientWebSocketUpgradesReturn404(t *testing.T) {
	server := httptest.NewServer(liteTestRouter())
	defer server.Close()
	dialer := websocket.Dialer{}
	for _, path := range []string{"/api/clients", "/api/clients/v1/rpc"} {
		for _, role := range []string{api.RoleGuest, api.RoleClient, api.RoleAdmin} {
			t.Run(path+"/"+role, func(t *testing.T) {
				header := http.Header{"Origin": {server.URL}, "X-Test-Role": {role}}
				conn, resp, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, header)
				if conn != nil {
					conn.Close()
					t.Fatal("legacy client endpoint accepted a WebSocket connection")
				}
				if resp == nil {
					t.Fatalf("missing HTTP response: %v", err)
				}
				defer resp.Body.Close()
				body, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if err != websocket.ErrBadHandshake {
					t.Fatalf("upgrade error = %v, want bad handshake", err)
				}
				assertDisabledResponse(t, resp.StatusCode, resp.Header, body)
			})
		}
	}
}

func TestLiveRouteContractsRemainRegistered(t *testing.T) {
	r := liteTestRouter()
	routes := make(map[string]bool)
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/clients/v2/rpc", "POST /api/clients/v2/rpc",
		"GET /api/rpc2", "POST /api/rpc2",
		"GET /api/public", "GET /api/me", "GET /api/task/ping", "GET /api/version",
		"POST /api/login", "POST /api/logout", "GET /api/admin/client/list",
	} {
		if !routes[route] {
			t.Errorf("live route disappeared: %s", route)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/clients/v2/rpc", nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s agent v2 route status = %d, want 401", method, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK || w.Body.String() != "pong" {
		t.Fatalf("ping contract changed: status=%d body=%q", w.Code, w.Body.String())
	}
}
