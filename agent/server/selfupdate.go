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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lyhbdw/komari-lite/agent/dnsresolver"
	"github.com/lyhbdw/komari-lite/agent/internal/boundedexec"
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

// parseVersion 严格解析点分数字版本号，允许一个合法的小写 v 前缀。
// 比较只解析版本数值，不改写下载路径中的原始版本标识。
func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(v, "v")
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

	// sha256：优先用事件内嵌值，否则从面板取 .sha256 文件。
	if expectedSHA == "" {
		expectedSHA, err = fetchSHA256WithRetry(base, targetVersion, asset)
		if err != nil {
			log.Printf("selfupdate: %v", err)
			return
		}
	}

	tmpName, err := downloadBinaryWithRetry(base, targetVersion, asset, filepath.Dir(exePath), expectedSHA)
	if err != nil {
		log.Printf("selfupdate: %v", err)
		return
	}
	defer os.Remove(tmpName)
	if err := smokeTestBinary(tmpName); err != nil {
		log.Printf("selfupdate: pre-flight smoke test failed (%v), keeping current agent", err)
		return
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		log.Printf("selfupdate: %v", err)
		return
	}

	log.Printf("selfupdate: replaced %s with %s, exiting for restart", exePath, targetVersion)
	// 交给 init 系统重启。非 systemd/upstart 环境（前台运行）下进程退出，
	// 用户需自行重启；install.sh 安装的服务均有 Restart=always / respawn。
	os.Exit(0)
}

// smokeTestBinary executes the candidate with bounded time and output.
func smokeTestBinary(path string) error {
	ctx := context.Background()
	out, err := boundedexec.Run(ctx, 3*time.Second, 64<<10, path, "--help")
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

// downloadCandidateURLs 构造多源下载候选列表：
// 1. 面板 CDN .bin 别名（优先命中 Cloudflare/CDN 缓存）
// 2. 面板直连原始路径
// 3. 加速国内 GitHub 镜像（ghfast.top, ghproxy.net）
// 4. GitHub Release 官方直链
func downloadCandidateURLs(base, targetVersion, asset string) []string {
	cleanVersion := strings.TrimPrefix(targetVersion, "v")
	urls := []string{
		fmt.Sprintf("%s/download/agent/%s/%s.bin", base, targetVersion, asset),
		fmt.Sprintf("%s/download/agent/%s/%s", base, targetVersion, asset),
	}
	for _, mirror := range []string{"https://ghfast.top", "https://ghproxy.net"} {
		urls = append(urls,
			fmt.Sprintf("%s/https://github.com/lyhbdw/komari-lite/releases/download/%s/%s", mirror, cleanVersion, asset),
			fmt.Sprintf("%s/https://github.com/lyhbdw/komari-lite/releases/download/v%s/%s", mirror, cleanVersion, asset),
		)
	}
	urls = append(urls,
		fmt.Sprintf("https://github.com/lyhbdw/komari-lite/releases/download/%s/%s", cleanVersion, asset),
		fmt.Sprintf("https://github.com/lyhbdw/komari-lite/releases/download/v%s/%s", cleanVersion, asset),
	)
	return urls
}

// downloadBinaryWithRetry 在网络抖动下尝试多源下载（面板 CDN、面板直连、国内加速镜像）。
func downloadBinaryWithRetry(base, targetVersion, asset, dir, expectedSHA string) (string, error) {
	candidates := downloadCandidateURLs(base, targetVersion, asset)
	var lastErr error
	for _, candidateURL := range candidates {
		bin, err := downloadBinary(candidateURL, dir, expectedSHA)
		if err == nil {
			return bin, nil
		}
		lastErr = err
		log.Printf("selfupdate: download from %s failed: %v", candidateURL, err)
	}
	return "", lastErr
}

// fetchSHA256 从面板或国内镜像多源获取 sha256 校验值。
func fetchSHA256(base, targetVersion, asset string) (string, error) {
	cleanVersion := strings.TrimPrefix(targetVersion, "v")
	urls := []string{
		fmt.Sprintf("%s/download/agent/%s/%s.bin.sha256", base, targetVersion, asset),
		fmt.Sprintf("%s/download/agent/%s/%s.sha256", base, targetVersion, asset),
	}
	for _, mirror := range []string{"https://ghfast.top", "https://ghproxy.net"} {
		urls = append(urls,
			fmt.Sprintf("%s/https://github.com/lyhbdw/komari-lite/releases/download/%s/%s.sha256", mirror, cleanVersion, asset),
			fmt.Sprintf("%s/https://github.com/lyhbdw/komari-lite/releases/download/v%s/%s.sha256", mirror, cleanVersion, asset),
		)
	}
	urls = append(urls,
		fmt.Sprintf("https://github.com/lyhbdw/komari-lite/releases/download/%s/%s.sha256", cleanVersion, asset),
		fmt.Sprintf("https://github.com/lyhbdw/komari-lite/releases/download/v%s/%s.sha256", cleanVersion, asset),
	)
	for _, u := range urls {
		if sum, err := fetchSingleSHA256(u); err == nil {
			return sum, nil
		}
	}
	return "", fmt.Errorf("failed to fetch sha256 from controller or mirrors")
}

func fetchSingleSHA256(url string) (string, error) {
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 513))
	if err != nil {
		return "", fmt.Errorf("fetch sha256: %w", err)
	}
	if len(body) > 512 {
		return "", fmt.Errorf("fetch sha256: response exceeds 512 bytes")
	}
	// 格式：<hex>  <filename>（sha256sum 输出格式）
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("fetch sha256: empty response")
	}
	return validateSHA256(fields[0])
}

const maxUpdateBinaryBytes int64 = 100 << 20

func validateSHA256(sum string) (string, error) {
	if len(sum) != sha256.Size*2 {
		return "", fmt.Errorf("invalid SHA256 length")
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", fmt.Errorf("invalid SHA256: %w", err)
	}
	return strings.ToLower(sum), nil
}

// downloadBinary streams into a same-directory staging file, never a binary-sized heap buffer.
func downloadBinary(url, dir, expectedSHA string) (string, error) {
	if _, err := validateSHA256(expectedSHA); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := updateHTTPClient(5 * time.Minute).Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxUpdateBinaryBytes {
		return "", fmt.Errorf("download exceeds %d bytes", maxUpdateBinaryBytes)
	}
	return stageBinary(resp.Body, dir, expectedSHA, maxUpdateBinaryBytes)
}

func stageBinary(body io.Reader, dir, expectedSHA string, limit int64) (path string, err error) {
	expectedSHA, err = validateSHA256(expectedSHA)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".agent-update-*")
	if err != nil {
		return "", err
	}
	path = tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmp.Name())
			path = ""
		}
	}()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(body, limit+1))
	if err != nil {
		return "", fmt.Errorf("download copy: %w", err)
	}
	if n > limit {
		return "", fmt.Errorf("download exceeds %d bytes", limit)
	}
	if n == 0 {
		return "", fmt.Errorf("download is empty")
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expectedSHA {
		return "", fmt.Errorf("sha256 mismatch (want %s, got %s)", expectedSHA, actual)
	}
	if err = tmp.Chmod(0o755); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	return path, nil
}
