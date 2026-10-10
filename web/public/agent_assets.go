package public

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

const (
	AgentAssetVersion = "v1.1.4"
	agentAssetDirEnv  = "KOMARI_AGENT_ASSET_DIR"
	defaultAssetDir   = "/app/agent-assets"
)

var allowedAgentAssets = map[string]string{
	"komari-agent-linux-amd64":        "application/octet-stream",
	"komari-agent-linux-amd64.sha256": "text/plain; charset=utf-8",
	"komari-agent-linux-arm64":        "application/octet-stream",
	"komari-agent-linux-arm64.sha256": "text/plain; charset=utf-8",
}

func isValidVersion(v string) bool {
	if len(v) == 0 || len(v) > 32 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' && r != '-' && r != '_' && r != 'v' && r != 'V' {
			return false
		}
	}
	return true
}

// ServeAgentAsset serves only the pinned Agent Lite release files mounted by
// the operator. It deliberately does not proxy GitHub or accept arbitrary
// paths, versions, or filenames.
func ServeAgentAsset(c *gin.Context) {
	reqVersion := c.Param("version")
	asset := c.Param("asset")
	contentType, allowed := allowedAgentAssets[asset]
	if !allowed {
		c.Status(http.StatusNotFound)
		return
	}

	targetVersion := reqVersion
	if reqVersion == "latest" {
		targetVersion = AgentAssetVersion
	} else if reqVersion != AgentAssetVersion {
		if !isValidVersion(reqVersion) {
			c.Status(http.StatusNotFound)
			return
		}
	}

	assetDir := os.Getenv(agentAssetDirEnv)
	if assetDir == "" {
		assetDir = defaultAssetDir
	}
	assetPath := filepath.Join(assetDir, targetVersion, asset)
	info, err := os.Lstat(assetPath)
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}

	file, err := os.Open(assetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.DataFromReader(http.StatusOK, info.Size(), contentType, file, nil)
}
