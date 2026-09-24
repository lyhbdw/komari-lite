package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/utils"
)

// admin.update.go
// 版本更新检查：对比上游 Lite 仓库（Tumb1er1376/komari-monitor-lite）的最新
// release 与当前运行的 KOMARI_VERSION，提示管理员有新版本可用。
// 只读、无副作用；网络不可达时返回错误而不是阻塞。

const updateCheckRepo = "Tumb1er1376/komari-monitor-lite"

var updateCheckHTTPClient = &http.Client{Timeout: 10 * time.Second}

func init() {
	RegisterWithGroupAndMeta("checkUpdate", rpc.RoleAdmin, adminCheckUpdate, &rpc.MethodMeta{
		Name:    "admin:checkUpdate",
		Summary: "Check the upstream Lite repository for a newer release",
		Returns: "{ current: string, latest: string, update_available: bool, release_url: string }",
	})
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func adminCheckUpdate(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", updateCheckRepo)
	resp, err := updateCheckHTTPClient.Get(url)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to reach GitHub API: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, rpc.MakeError(rpc.InternalError, fmt.Sprintf("GitHub API returned status %d", resp.StatusCode), nil)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to parse release info: "+err.Error(), nil)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := utils.CurrentVersion
	updateAvailable := latest != "" && latest != current

	return map[string]any{
		"current":          current,
		"latest":          latest,
		"update_available": updateAvailable,
		"release_url":     release.HTMLURL,
	}, nil
}
