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
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
	v2 "github.com/lyhbdw/komari-lite/agent/protocol/v2"
	"github.com/lyhbdw/komari-lite/agent/version"
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

// versionLess 比较两个点分版本号：a < b 返回 true。
// 任一段解析失败（非纯数字）时按字符串比较，避免畸形版本号被
// Sscanf 宽松解析成意外数值（如 "1.0.5x" 被当作 5）。
func versionLess(a, b string) bool {
	ai, aok := parseVersion(a)
	bi, bok := parseVersion(b)
	if !aok || !bok {
		// 当前运行快照/开发版本，而目标版本是正式点分版本时，正式版本始终被视为更新（不小于当前）
		if aok && !bok && strings.HasPrefix(strings.ToLower(b), "snapshot") {
			return false
		}
		return a < b
	}
	for i := 0; i < len(ai) && i < len(bi); i++ {
		if ai[i] != bi[i] {
			return ai[i] < bi[i]
		}
	}
	return len(ai) < len(bi)
}

// parseVersion 严格解析点分数字版本号；任何一段非纯数字则 ok=false。
func parseVersion(v string) ([]int, bool) {
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, s := range parts {
		if s == "" {
			return nil, false
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return nil, false
			}
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
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
		expectedSHA, err = fetchSHA256WithRetry(base, targetVersion, asset)
		if err != nil {
			log.Printf("selfupdate: %v", err)
			return
		}
	}

	bin, err := downloadBinaryWithRetry(binURL)
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

	// 升级预检（Smoke Test）：在执行真正替换前，启动临时文件做一次健康探测
	// 验证目标环境对该二进制的架构、链接器（glibc/musl）、UPX 解压以及执行权限是否正常支持
	// 若探活失败，保留旧版本继续运行，杜绝节点因异常二进制而失联
	if err := smokeTestBinary(tmpName); err != nil {
		os.Remove(tmpName)
		log.Printf("selfupdate: pre-flight smoke test failed (%v), aborting update to protect agent", err)
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

// smokeTestBinary 执行短生命周期探活，确保下载的文件能在当前机器上真正执行
func smokeTestBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--help")
	cmd.Env = append(os.Environ(), "KOMARI_SMOKE_TEST=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exit error: %w, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// updateHTTPClient 返回升级下载专用 client：走 dnsresolver 的 IP 偏好栈
// （--prefer-ip-version 生效），超时按下载阶段区分。
func updateHTTPClient(timeout time.Duration) *http.Client {
	return dnsresolver.GetHTTPClientWithPreference(timeout, flags.PreferIPVersion)
}

// fetchSHA256WithRetry 在网络抖动下重试获取 sha256（节点到面板的链路
// 可能不稳定，单次失败直接放弃会让升级事件被白白消费）。
func fetchSHA256WithRetry(base, targetVersion, asset string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		sha, err := fetchSHA256(base, targetVersion, asset)
		if err == nil {
			return sha, nil
		}
		lastErr = err
		log.Printf("selfupdate: fetch sha256 attempt %d failed: %v", attempt, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * 10 * time.Second)
		}
	}
	return "", lastErr
}

// downloadBinaryWithRetry 在网络抖动下重试下载（约 8MB，弱网节点
// 一次 TLS 握手超时很常见）。
func downloadBinaryWithRetry(url string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		bin, err := downloadBinary(url)
		if err == nil {
			return bin, nil
		}
		lastErr = err
		log.Printf("selfupdate: download attempt %d failed: %v", attempt, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * 10 * time.Second)
		}
	}
	return nil, lastErr
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
	resp, err := updateHTTPClient(30 * time.Second).Do(req)
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
	resp, err := updateHTTPClient(5 * time.Minute).Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: status %d", resp.StatusCode)
	}
	// 限制最大下载 100MB，防止异常响应打满内存
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
}
