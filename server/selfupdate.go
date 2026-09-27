package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	v2 "github.com/Tumb1er1376/komari-agent-lite/protocol/v2"
	"github.com/Tumb1er1376/komari-agent-lite/version"
)

// selfUpdateMu 防止并发自升级。
var selfUpdateMu sync.Mutex

// selfupdate.go
// agent.update 事件处理：从面板下载新版本二进制，sha256 校验后原子自替换，
// 然后 exit(0) 交给 systemd (Restart=always) / upstart (respawn) 拉起新版本。
//
// 安全边界：
//   - 只从 agent 自己连接的面板下载（Endpoint 同源），不信任任何第三方源。
//   - sha256 必须匹配面板提供的 .sha256 文件（或事件内嵌值）才落盘生效。
//   - 下载到同目录临时文件后 rename，失败不影响正在运行的旧版本。

// assetName 返回当前平台的资产文件名，与服务端 agent-assets 目录布局一致。
func assetName() string {
	return fmt.Sprintf("komari-agent-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// versionLess 比较两个点分版本号：a < b 返回 true。解析失败时按字符串比较。
func versionLess(a, b string) bool {
	var ai, bi []int
	for _, s := range strings.Split(a, ".") {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			return a < b
		}
		ai = append(ai, n)
	}
	for _, s := range strings.Split(b, ".") {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			return a < b
		}
		bi = append(bi, n)
	}
	for i := 0; i < len(ai) && i < len(bi); i++ {
		if ai[i] != bi[i] {
			return ai[i] < bi[i]
		}
	}
	return len(ai) < len(bi)
}

// HandleAgentUpdate 处理 agent.update 事件。返回 true 表示事件已接受
// （无论升级是否最终成功，事件本身都被消费掉）。
func HandleAgentUpdate(params interface{}) bool {
	var p v2.UpdateParams
	if err := v2.BindParams(params, &p); err != nil || p.Version == "" {
		log.Printf("selfupdate: bad update params: %v", err)
		return false
	}

	if p.Version == version.Current {
		log.Printf("selfupdate: already on %s, ignoring", p.Version)
		return true
	}
	if versionLess(p.Version, version.Current) {
		log.Printf("selfupdate: target %s is older than current %s, ignoring", p.Version, version.Current)
		return true
	}

	go performSelfUpdate(p.Version, p.SHA256)
	return true
}

// performSelfUpdate 下载、校验并替换自身，成功后退出进程。
func performSelfUpdate(targetVersion, expectedSHA string) {
	// 防止并发升级（事件去重已挡掉大部分，这里再兜底）。
	if !selfUpdateMu.TryLock() {
		return
	}
	defer selfUpdateMu.Unlock()

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("selfupdate: cannot resolve executable: %v", err)
		return
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		log.Printf("selfupdate: cannot resolve symlinks: %v", err)
		return
	}

	log.Printf("selfupdate: %s -> %s", version.Current, targetVersion)

	base := strings.TrimSuffix(flags.Endpoint, "/")
	asset := assetName()
	binURL := fmt.Sprintf("%s/download/agent/%s/%s", base, targetVersion, asset)

	// sha256：优先用事件内嵌值，否则从面板取 .sha256 文件。
	if expectedSHA == "" {
		expectedSHA, err = fetchSHA256(base, targetVersion, asset)
		if err != nil {
			log.Printf("selfupdate: %v", err)
			return
		}
	}

	bin, err := downloadBinary(binURL)
	if err != nil {
		log.Printf("selfupdate: %v", err)
		return
	}

	sum := sha256.Sum256(bin)
	actual := hex.EncodeToString(sum[:])
	if actual != expectedSHA {
		log.Printf("selfupdate: sha256 mismatch (want %s, got %s), aborting", expectedSHA, actual)
		return
	}

	// 写同目录临时文件 → chmod → rename 原子替换。
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".agent-update-*")
	if err != nil {
		log.Printf("selfupdate: %v", err)
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		log.Printf("selfupdate: %v", err)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		log.Printf("selfupdate: %v", err)
		return
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		log.Printf("selfupdate: %v", err)
		return
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		os.Remove(tmpName)
		log.Printf("selfupdate: %v", err)
		return
	}

	log.Printf("selfupdate: replaced %s with %s, exiting for restart", exePath, targetVersion)
	// 交给 init 系统重启。非 systemd/upstart 环境（前台运行）下进程退出，
	// 用户需自行重启；install.sh 安装的服务均有 Restart=always / respawn。
	os.Exit(0)
}

// fetchSHA256 从面板下载 <asset>.sha256 文件并解析出十六进制摘要。
func fetchSHA256(base, targetVersion, asset string) (string, error) {
	url := fmt.Sprintf("%s/download/agent/%s/%s.sha256", base, targetVersion, asset)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch sha256: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch sha256: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", fmt.Errorf("fetch sha256: %w", err)
	}
	// 格式：<hex>  <filename>（sha256sum 输出格式）
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("fetch sha256: empty response")
	}
	sum := fields[0]
	if _, err := hex.DecodeString(sum); err != nil {
		return "", fmt.Errorf("fetch sha256: bad digest %q", sum)
	}
	return sum, nil
}

// downloadBinary 下载新版本二进制到内存（约 8MB）。
func downloadBinary(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
