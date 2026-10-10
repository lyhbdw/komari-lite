package public

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAgentInstallScriptDefaultIsLatest guards against reintroducing a
// hardcoded semver default in agent-install.sh. The controller only serves
// AgentAssetVersion and "latest"; a pinned default like "1.0.7" 404s as soon
// as the hosted asset moves on, breaking one-click installs.
func TestAgentInstallScriptDefaultIsLatest(t *testing.T) {
	script, err := os.ReadFile("agent-install.sh")
	if err != nil {
		t.Fatal(err)
	}
	hardcodedDefault := regexp.MustCompile(`(?m)^\s*version_to_install="[0-9]+\.[0-9]+`)
	if loc := hardcodedDefault.FindIndex(script); loc != nil {
		line := strings.SplitN(string(script[:loc[0]]), "\n", -1)
		t.Fatalf("agent-install.sh hardcodes a version default at line %d; use \"latest\" so the controller resolves it: %q",
			len(line), strings.TrimSpace(string(script[loc[0]:loc[1]+40])))
	}
}

func TestServeAgentAssetAllowsOnlyPinnedFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetDir := t.TempDir()
	versionDir := filepath.Join(assetDir, AgentAssetVersion)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := []byte("verified-agent-binary")
	checksum := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  komari-agent-linux-amd64\n")
	if err := os.WriteFile(filepath.Join(versionDir, "komari-agent-linux-amd64"), binary, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "komari-agent-linux-amd64.sha256"), checksum, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(agentAssetDirEnv, assetDir)

	router := gin.New()
	router.GET("/download/agent/:version/:asset", ServeAgentAsset)

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantBody    []byte
		contentType string
	}{
		{
			name:        "binary",
			path:        "/download/agent/" + AgentAssetVersion + "/komari-agent-linux-amd64",
			wantStatus:  http.StatusOK,
			wantBody:    binary,
			contentType: "application/octet-stream",
		},
		{
			name:        "binary .bin alias",
			path:        "/download/agent/" + AgentAssetVersion + "/komari-agent-linux-amd64.bin",
			wantStatus:  http.StatusOK,
			wantBody:    binary,
			contentType: "application/octet-stream",
		},
		{
			name:        "binary latest",
			path:        "/download/agent/latest/komari-agent-linux-amd64",
			wantStatus:  http.StatusOK,
			wantBody:    binary,
			contentType: "application/octet-stream",
		},
		{
			name:        "checksum",
			path:        "/download/agent/" + AgentAssetVersion + "/komari-agent-linux-amd64.sha256",
			wantStatus:  http.StatusOK,
			wantBody:    checksum,
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:        "checksum .bin.sha256 alias",
			path:        "/download/agent/" + AgentAssetVersion + "/komari-agent-linux-amd64.bin.sha256",
			wantStatus:  http.StatusOK,
			wantBody:    checksum,
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:       "missing allowed asset",
			path:       "/download/agent/" + AgentAssetVersion + "/komari-agent-linux-arm64",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "old version",
			path:       "/download/agent/1.0.3/komari-agent-linux-amd64",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unknown asset",
			path:       "/download/agent/" + AgentAssetVersion + "/SHA256SUMS",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "encoded traversal",
			path:       "/download/agent/" + AgentAssetVersion + "/%2e%2e",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", response.Code, tt.wantStatus, response.Body.String())
			}
			if tt.wantBody != nil && string(response.Body.Bytes()) != string(tt.wantBody) {
				t.Fatalf("body = %q, want %q", response.Body.Bytes(), tt.wantBody)
			}
			if tt.contentType != "" && response.Header().Get("Content-Type") != tt.contentType {
				t.Fatalf("Content-Type = %q, want %q", response.Header().Get("Content-Type"), tt.contentType)
			}
			if tt.wantStatus == http.StatusOK && response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}
