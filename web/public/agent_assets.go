package public

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/clients"
	"github.com/lyhbdw/komari-lite/internal/config"
)

const (
	AgentAssetVersion = "v1.1.7"
	agentAssetDirEnv  = "KOMARI_AGENT_ASSET_DIR"
	defaultAssetDir   = "/app/agent-assets"
)

var allowedAgentAssets = map[string]string{
	"komari-agent-linux-amd64":            "application/octet-stream",
	"komari-agent-linux-amd64.sha256":     "text/plain; charset=utf-8",
	"komari-agent-linux-amd64.bin":        "application/octet-stream",
	"komari-agent-linux-amd64.bin.sha256": "text/plain; charset=utf-8",
	"komari-agent-linux-arm64":            "application/octet-stream",
	"komari-agent-linux-arm64.sha256":     "text/plain; charset=utf-8",
	"komari-agent-linux-arm64.bin":        "application/octet-stream",
	"komari-agent-linux-arm64.bin.sha256": "text/plain; charset=utf-8",
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
	if err != nil && strings.HasSuffix(asset, ".bin") {
		assetPath = filepath.Join(assetDir, targetVersion, strings.TrimSuffix(asset, ".bin"))
		info, err = os.Lstat(assetPath)
	} else if err != nil && strings.HasSuffix(asset, ".bin.sha256") {
		assetPath = filepath.Join(assetDir, targetVersion, strings.TrimSuffix(asset, ".bin.sha256")+".sha256")
		info, err = os.Lstat(assetPath)
	}
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
	c.Header("CDN-Cache-Control", "public, max-age=31536000")
	c.Header("Cloudflare-CDN-Cache-Control", "public, max-age=31536000")
	c.DataFromReader(http.StatusOK, info.Size(), contentType, file, nil)
}

// IsValidToken validates that the token string contains only alphanumeric
// and URL/safe characters and is of reasonable length.
func IsValidToken(token string) bool {
	if len(token) < 8 || len(token) > 128 {
		return false
	}
	for _, r := range token {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// ResolveEndpoint determines the public URL endpoint for Komari agents.
// It prioritizes the explicitly configured script_domain, then falls back to the
// incoming request's scheme and host.
func ResolveEndpoint(c *gin.Context) string {
	if domain, err := config.GetAs[string]("script_domain", ""); err == nil {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
				return strings.TrimRight(domain, "/")
			}
			return "https://" + strings.TrimRight(domain, "/")
		}
	}
	proto := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" || c.GetHeader("Cf-Visitor") != "" {
		proto = "https"
	}
	host := c.Request.Host
	if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	return proto + "://" + strings.TrimRight(host, "/")
}

// ServeDynamicInstallScript serves a tailored agent install script with AGENT_ENDPOINT
// and AGENT_TOKEN dynamically embedded.
func ServeDynamicInstallScript(c *gin.Context) {
	rawToken := c.Param("token")
	token := strings.TrimSpace(strings.TrimSuffix(rawToken, ".sh"))
	if !IsValidToken(token) {
		c.String(http.StatusBadRequest, "Invalid token format\n")
		return
	}

	// Verify token exists in database
	uuid, err := clients.GetClientUUIDByToken(token)
	if err != nil || uuid == "" {
		c.String(http.StatusNotFound, "Node not found\n")
		return
	}

	endpoint := ResolveEndpoint(c)
	header := fmt.Sprintf("#!/bin/sh\n# Tailored installation script for Komari Agent\nAGENT_ENDPOINT=%s\nAGENT_TOKEN=%s\n\n", shellQuote(endpoint), shellQuote(token))

	baseScript := AgentInstallScript
	if idx := bytes.IndexByte(baseScript, '\n'); idx != -1 {
		baseScript = baseScript[idx+1:]
	}

	tailored := append([]byte(header), baseScript...)

	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Data(http.StatusOK, "text/x-shellscript; charset=utf-8", tailored)
}
