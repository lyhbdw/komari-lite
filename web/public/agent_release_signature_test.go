package public

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeAgentReleaseSignaturesAndBinAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	versionDir := filepath.Join(dir, AgentAssetVersion)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(agentAssetDirEnv, dir)
	router := gin.New()
	router.GET("/download/agent/:version/:asset", ServeAgentAsset)
	for _, arch := range []string{"amd64", "arm64"} {
		asset := "komari-agent-linux-" + arch
		data := []byte(`{"version":"1.1.8","asset":"` + asset + `","sha256":"test","key_id":"test","signature":"fixture"}`)
		if err := os.WriteFile(filepath.Join(versionDir, asset+".sig.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{".sig.json", ".bin.sig.json"} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/download/agent/latest/"+asset+suffix, nil))
			if response.Code != 200 || response.Body.String() != string(data) {
				t.Fatalf("signature %s%s: %d %s", asset, suffix, response.Code, response.Body.String())
			}
		}
	}
}
