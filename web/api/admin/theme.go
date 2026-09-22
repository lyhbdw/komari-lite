package admin

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/public"
)

const (
	maxThemeArchiveFiles  = 10000
	maxThemeFileSize      = 128 << 20
	maxThemeExtractedSize = 512 << 20
	maxThemeManifestSize  = 1 << 20
)

var themeInstallMu sync.Mutex

// ListThemes 列出所有主题
func ListThemes(c *gin.Context) {
	dataDir := "./data/theme"

	// 确保主题目录存在
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			api.RespondError(c, http.StatusInternalServerError, "创建主题目录失败: "+err.Error())
			return
		}
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "读取主题目录失败: "+err.Error())
		return
	}

	var themes []models.Theme
	defaultTheme, err := public.PublicFS.ReadFile("defaultTheme/komari-theme.json")
	if err == nil {
		dt := models.Theme{}
		err := json.Unmarshal(defaultTheme, &dt)
		if err == nil && dt.Short == public.DefaultTheme {
			themes = append(themes, dt)
		}

	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == public.DefaultTheme {
			themeConfigPath := filepath.Join(dataDir, entry.Name(), "komari-theme.json")
			if themeInfo, err := loadThemeConfig(themeConfigPath); err == nil && themeInfo.Short == public.DefaultTheme {
				themes = []models.Theme{themeInfo}
			}
		}
	}

	api.RespondSuccess(c, themes)
}

// DeleteTheme 删除主题
func DeleteTheme(c *gin.Context) {
	var req struct {
		Short string `json:"short" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if req.Short == public.DefaultTheme {
		api.RespondError(c, http.StatusBadRequest, "默认主题不能删除")
		return
	}

	// 校验主题短名称，防止路径穿越（如 ../）导致删除工作目录外的任意文件
	if !isValidMarketShort(req.Short) {
		api.RespondError(c, http.StatusBadRequest, "无效的主题名称")
		return
	}

	themeDir := filepath.Join("./data/theme", req.Short)

	// 检查主题是否存在
	if _, err := os.Stat(themeDir); os.IsNotExist(err) {
		api.RespondError(c, http.StatusNotFound, "主题不存在")
		return
	}

	// 删除主题目录
	if err := os.RemoveAll(themeDir); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "删除主题失败: "+err.Error())
		return
	}

	api.RespondSuccessMessage(c, "主题删除成功", nil)
}

// SetTheme 设置主题
func SetTheme(c *gin.Context) {
	// CSRF 防御：状态变更端点仅接受 POST（原为 GET，可被跨站 <img> 触发）。
	if c.Request.Method != http.MethodPost {
		api.RespondError(c, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	// 兼容 query 与 JSON body 两种传参方式。
	themeName := c.Query("theme")
	if themeName == "" {
		var body struct {
			Theme string `json:"theme"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			themeName = body.Theme
		}
	}
	if themeName == "" {
		api.RespondError(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}

	if themeName != public.DefaultTheme {
		api.RespondError(c, http.StatusBadRequest, "仅支持 Komari Emerald 主题")
		return
	}

	if err := config.Set("theme", themeName); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "更新主题设置失败: "+err.Error())
		return
	}

	api.RespondSuccessMessage(c, "主题设置成功", gin.H{"theme": themeName})
}

// extractAndValidateTheme 解压并验证主题
func extractAndValidateTheme(zipPath string) (models.Theme, error) {
	themeInstallMu.Lock()
	defer themeInstallMu.Unlock()
	var themeInfo models.Theme

	// 打开ZIP文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return themeInfo, fmt.Errorf("无法打开ZIP文件: %v", err)
	}
	defer r.Close()

	if err := validateThemeArchive(r.File); err != nil {
		return themeInfo, err
	}

	// 查找komari-theme.json文件
	var themeConfigFile *zip.File
	for _, f := range r.File {
		if f.Name == "komari-theme.json" {
			themeConfigFile = f
			break
		}
	}

	if themeConfigFile == nil {
		return themeInfo, fmt.Errorf("主题配置文件 komari-theme.json 不存在")
	}

	// 读取主题配置
	rc, err := themeConfigFile.Open()
	if err != nil {
		return themeInfo, fmt.Errorf("无法读取主题配置文件: %v", err)
	}
	defer rc.Close()

	configData, err := io.ReadAll(io.LimitReader(rc, maxThemeManifestSize+1))
	if err != nil {
		return themeInfo, fmt.Errorf("读取主题配置失败: %v", err)
	}
	if len(configData) > maxThemeManifestSize {
		return themeInfo, fmt.Errorf("主题配置文件超过 %d 字节限制", maxThemeManifestSize)
	}

	if err := json.Unmarshal(configData, &themeInfo); err != nil {
		return themeInfo, fmt.Errorf("主题配置格式错误: %v", err)
	}

	if err := validateThemeManifest(themeInfo); err != nil {
		return themeInfo, err
	}

	// Extract into a sibling staging directory, then atomically replace the
	// current theme so a corrupt package never destroys the working copy.
	themeDir := filepath.Join("./data/theme", themeInfo.Short)
	stagingDir, err := os.MkdirTemp(filepath.Dir(themeDir), ".Emerald-update-")
	if err != nil {
		return themeInfo, fmt.Errorf("创建主题目录失败: %v", err)
	}
	defer os.RemoveAll(stagingDir)

	// 解压文件到主题目录
	for _, f := range r.File {
		path := filepath.Join(stagingDir, f.Name)

		// 安全检查，防止路径遍历攻击
		if !strings.HasPrefix(path, filepath.Clean(stagingDir)+string(os.PathSeparator)) {
			return themeInfo, fmt.Errorf("主题压缩包包含无效路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			// 目录固定 0755，剥除压缩包内记录的任何附加位（如 setgid）。
			os.MkdirAll(path, 0755)
			continue
		}

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return themeInfo, fmt.Errorf("创建目录失败: %v", err)
		}

		// 解压文件
		rc, err := f.Open()
		if err != nil {
			return themeInfo, fmt.Errorf("打开压缩文件失败: %v", err)
		}

		// 文件 mode 白名单化：固定 0644，剥除 setuid/setgid/可执行等
		// 压缩包内记录的权限位（web 根目录内容无需可执行）。
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return themeInfo, fmt.Errorf("创建文件失败: %v", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return themeInfo, fmt.Errorf("解压文件失败: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(stagingDir, "dist", "index.html")); err != nil {
		return themeInfo, fmt.Errorf("主题缺少 dist/index.html")
	}
	backupDir := themeDir + ".previous"
	_ = os.RemoveAll(backupDir)
	if _, err := os.Stat(themeDir); err == nil {
		if err := os.Rename(themeDir, backupDir); err != nil {
			return themeInfo, fmt.Errorf("备份原有主题失败: %v", err)
		}
	}
	if err := os.Rename(stagingDir, themeDir); err != nil {
		_ = os.Rename(backupDir, themeDir)
		return themeInfo, fmt.Errorf("安装主题失败: %v", err)
	}
	_ = os.RemoveAll(backupDir)

	return themeInfo, nil
}

func validateThemeArchive(files []*zip.File) error {
	if len(files) > maxThemeArchiveFiles {
		return fmt.Errorf("主题压缩包文件数量超过 %d 个限制", maxThemeArchiveFiles)
	}
	var total uint64
	for _, file := range files {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if name == "" || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\x00') {
			return fmt.Errorf("主题压缩包包含无效路径: %s", file.Name)
		}
		for _, part := range strings.Split(name, "/") {
			if part == ".." || part == "." {
				return fmt.Errorf("主题压缩包包含无效路径: %s", file.Name)
			}
		}
		mode := file.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return fmt.Errorf("主题压缩包包含不支持的文件类型: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > maxThemeFileSize {
			return fmt.Errorf("主题文件 %s 超过 %d 字节限制", file.Name, maxThemeFileSize)
		}
		total += file.UncompressedSize64
		if total > maxThemeExtractedSize {
			return fmt.Errorf("主题解压后总大小超过 %d 字节限制", maxThemeExtractedSize)
		}
	}
	return nil
}

// loadThemeConfig 加载主题配置
func loadThemeConfig(configPath string) (models.Theme, error) {
	var themeInfo models.Theme

	data, err := os.ReadFile(configPath)
	if err != nil {
		return themeInfo, err
	}

	if err := json.Unmarshal(data, &themeInfo); err != nil {
		return themeInfo, err
	}

	return themeInfo, nil
}

func validateThemeManifest(themeInfo models.Theme) error {
	if !models.IsLocalizedText(themeInfo.Name) || themeInfo.Short == "" {
		return fmt.Errorf("主题配置缺少必填字段（name、short）")
	}
	if !isValidMarketShort(themeInfo.Short) {
		return fmt.Errorf("主题short字段格式无效，只允许字母、数字、下划线和连字符")
	}
	if themeInfo.Short != public.DefaultTheme {
		return fmt.Errorf("仅支持 Komari Emerald 主题")
	}
	return themeInfo.ValidateConfiguration()
}

// isValidMarketShort validates a market entry short name (shared by the
// theme and plugin markets).
func isValidMarketShort(short string) bool {
	if short == "" || short == "default" || short != public.DefaultTheme {
		return false
	}

	for _, r := range short {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}

	return true
}

func downloadThemeFromURL(rawURL string) ([]byte, error) {
	return DownloadMarketURL(rawURL, marketDownloadMaxBytes)
}

// getGitHubReleaseDownloadURL 从GitHub API获取最新release的下载链接
// 该函数通过GitHub API获取指定仓库最新release的资源下载链接
// 参考API: https://api.github.com/repos/{owner}/{repo}/releases/latest
// 参数:
//   - owner: GitHub仓库所有者
//   - repo: GitHub仓库名称
//
// 返回:
//   - 最新release的第一个资源的下载链接
//   - 错误信息（如果有）
func getGitHubReleaseDownloadURL(owner, repo string) (string, error) {
	if owner == "" || repo == "" {
		return "", errors.New("GitHub仓库所有者和仓库名称不能为空")
	}
	// 校验 owner/repo 字符集，防止注入路径段或查询参数（如 "a/b?x=" 或 "../api"）。
	if !isValidGitHubName(owner) || !isValidGitHubName(repo) {
		return "", errors.New("GitHub仓库所有者或仓库名称包含非法字符")
	}

	// 构建GitHub API URL
	// 使用GitHub API获取最新release信息
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	data, err := DownloadMarketURL(apiURL, maxThemeManifestSize)
	if err != nil {
		return "", fmt.Errorf("获取GitHub release信息失败: %v", err)
	}

	var releaseInfo struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.Unmarshal(data, &releaseInfo); err != nil {
		return "", fmt.Errorf("解析GitHub API响应失败: %v", err)
	}

	// 检查是否有可下载的资源
	if len(releaseInfo.Assets) == 0 {
		return "", errors.New("GitHub release中没有可下载的资源")
	}

	// 选择资产：优先匹配主题名的 zip，其次任意 .zip 后缀；不再盲取第一个
	// （release 常含 sha256/签名等非主题资产）。
	target := ""
	for _, asset := range releaseInfo.Assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".zip") {
			if strings.Contains(name, strings.ToLower(public.DefaultTheme)) {
				target = asset.BrowserDownloadURL
				break
			}
			if target == "" {
				target = asset.BrowserDownloadURL
			}
		}
	}
	if target == "" {
		return "", errors.New("GitHub release中没有可下载的 zip 资源")
	}
	return target, nil
}

// isValidGitHubName 校验 GitHub 用户名/仓库名：仅字母数字与 - _ .，且不得以 . 开头/结尾。
func isValidGitHubName(name string) bool {
	if name == "" || len(name) > 100 || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// isGitHubRepoURL 检查URL是否是GitHub仓库地址
// 支持的格式:
// - https://github.com/owner/repo
// - https://github.com/owner/repo.git
// - https://www.github.com/owner/repo
// - http://github.com/owner/repo
// 返回:
//   - 是否是GitHub仓库URL
//   - 仓库所有者
//   - 仓库名称
func isGitHubRepoURL(urlStr string) (bool, string, string) {
	if urlStr == "" {
		return false, "", ""
	}

	// 检查URL是否包含github.com
	if !strings.Contains(strings.ToLower(urlStr), "github.com") {
		return false, "", ""
	}

	// 解析URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false, "", ""
	}

	// 检查主机名是否是github.com或www.github.com
	hostname := strings.ToLower(parsedURL.Host)
	if hostname != "github.com" && hostname != "www.github.com" {
		return false, "", ""
	}

	// 解析路径部分，提取owner和repo
	// 路径格式应该是 /owner/repo 或 /owner/repo.git
	path := strings.TrimPrefix(parsedURL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return false, "", ""
	}

	owner := parts[0]
	repo := parts[1]

	// 如果repo以.git结尾，去掉这个后缀
	repo = strings.TrimSuffix(repo, ".git")

	return true, owner, repo
}

func downloadThemeSource(rawURL string) ([]byte, error) {
	if ok, owner, repo := isGitHubRepoURL(rawURL); ok {
		assetURL, err := getGitHubReleaseDownloadURL(owner, repo)
		if err != nil {
			return nil, err
		}
		rawURL = assetURL
	}
	return downloadThemeFromURL(rawURL)
}

func updateThemeFromBytes(c *gin.Context, data []byte) {
	tempFile, err := os.CreateTemp("./data/theme", ".Emerald-download-*.zip")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	if err := tempFile.Close(); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	manifest, err := extractAndValidateTheme(tempFile.Name())
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	api.RespondSuccessMessage(c, "主题更新成功", manifest)
}

// UpdateTheme 更新主题
// 支持四种更新方式：
// 1. 使用主题原有URL下载更新
// 2. 提供新的直接下载URL进行更新
// 3. 提供GitHub仓库信息，从最新release下载更新
// 4. 如果主题URL是GitHub仓库地址，自动获取最新release
func UpdateTheme(c *gin.Context) {
	var req struct {
		Short    string `json:"short" binding:"required"` // 主题短名称
		URL      string `json:"url"`                      // 新的URL地址（可选）
		GitOwner string `json:"git_owner"`                // GitHub仓库所有者（可选）
		GitRepo  string `json:"git_repo"`                 // GitHub仓库名称（可选）
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 校验主题短名称，防止路径穿越（如 ../）访问工作目录外的文件
	if !isValidMarketShort(req.Short) {
		api.RespondError(c, http.StatusBadRequest, "无效的主题名称")
		return
	}

	// 仅允许更新 Emerald；没有本地覆盖时使用嵌入式 Emerald 基线。
	if req.Short != public.DefaultTheme {
		api.RespondError(c, http.StatusBadRequest, "仅支持 Komari Emerald 主题")
		return
	}
	themeDir := filepath.Join("./data/theme", req.Short)
	themeConfigPath := filepath.Join(themeDir, "komari-theme.json")

	// 加载现有主题配置
	themeInfo, err := loadThemeConfig(themeConfigPath)
	if os.IsNotExist(err) {
		data, readErr := public.PublicFS.ReadFile("defaultTheme/komari-theme.json")
		if readErr == nil {
			err = json.Unmarshal(data, &themeInfo)
		}
	}
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "读取主题配置失败: "+err.Error())
		return
	}

	if req.URL != "" {
		result, err := downloadThemeSource(req.URL)
		if err != nil {
			api.RespondError(c, http.StatusBadRequest, "下载主题失败: "+err.Error())
			return
		}
		updateThemeFromBytes(c, result)
		return
	}

	// 方式1和方式4: 尝试从原始URL下载主题
	// 如果原始URL是GitHub仓库地址，则自动获取最新release
	var themeData []byte
	// 不保存下载链接，更新后由主题覆盖
	//var downloadURL string
	// var err2 error

	if themeInfo.URL != "" {
		// 检查原始URL是否是GitHub仓库地址
		// 例如: https://github.com/owner/repo
		isGitHub, owner, repo := isGitHubRepoURL(themeInfo.URL)
		if isGitHub {
			// 方式4: 如果原始URL是GitHub仓库地址，自动获取最新release
			// 这是本次需求的核心功能：当主题文件中现有的url地址如果是github仓库的路径，则直接引用该url地址去下载最新的release
			gitHubURL, err := getGitHubReleaseDownloadURL(owner, repo)
			if err == nil {
				// 使用获取到的GitHub release下载链接下载主题
				themeData, _ = downloadThemeFromURL(gitHubURL)
				//if err2 == nil {
				// 注意：这里我们保存的是release的下载链接，而不是GitHub仓库地址
				// 这样做是为了在下载成功后，将这个具体的release下载链接保存到主题配置中
				// 但在下次更新时，我们仍然会检测到这是一个GitHub仓库，并获取最新的release
				// downloadURL = gitHubURL
				//}
			}
		} else {
			// 原始URL不是GitHub仓库地址，直接尝试下载（方式1）
			themeData, _ = downloadThemeFromURL(themeInfo.URL)
			//if err2 == nil {
			// downloadURL = themeInfo.URL
			//}
		}
	}

	// 如果原始URL下载失败，尝试其他方式下载
	if themeData == nil || len(themeData) == 0 {
		// 方式3: 如果提供了GitHub仓库信息，尝试从GitHub最新release下载
		// 这种方式允许用户只需提供owner和repo信息，系统会自动获取最新release的下载链接
		if req.GitOwner != "" && req.GitRepo != "" {
			// 从GitHub API获取下载链接
			// 相当于: DOWNLOAD_URL=$(curl -s https://api.github.com/repos/owner/repo/releases/latest | jq -r ".assets[0].browser_download_url")
			gitHubURL, err := getGitHubReleaseDownloadURL(req.GitOwner, req.GitRepo)
			if err != nil {
				api.RespondError(c, http.StatusBadRequest, "从GitHub获取下载链接失败: "+err.Error())
				return
			}

			// 使用获取到的链接下载主题
			themeData, err = downloadThemeFromURL(gitHubURL)
			if err != nil {
				api.RespondError(c, http.StatusBadRequest, "从GitHub下载主题失败: "+err.Error())
				return
			}
			// 保存下载链接，稍后更新到主题配置中
			// downloadURL = gitHubURL
		} else if req.URL != "" {
			// 方式2: 如果提供了新URL，尝试从新URL下载
			// 检查新URL是否是GitHub仓库地址
			isGitHub, owner, repo := isGitHubRepoURL(req.URL)
			if isGitHub {
				// 如果新URL是GitHub仓库地址，获取最新release
				// 这里也应用了自动检测GitHub仓库并下载最新release的功能
				gitHubURL, err := getGitHubReleaseDownloadURL(owner, repo)
				if err != nil {
					api.RespondError(c, http.StatusBadRequest, "从GitHub获取下载链接失败: "+err.Error())
					return
				}

				// 使用获取到的链接下载主题
				themeData, err = downloadThemeFromURL(gitHubURL)
				if err != nil {
					api.RespondError(c, http.StatusBadRequest, "从GitHub下载主题失败: "+err.Error())
					return
				}
				// 保存GitHub仓库URL，而不是release下载链接，以便将来可以获取最新版本
				// 这是一个重要的设计决策：我们保存的是GitHub仓库URL，而不是具体的release下载链接
				// 这样在下次更新时，系统会再次检测到这是GitHub仓库，并自动获取最新的release
				// downloadURL = req.URL
			} else {
				// 新URL不是GitHub仓库地址，直接尝试下载
				themeData, err = downloadThemeFromURL(req.URL)
				if err != nil {
					api.RespondError(c, http.StatusBadRequest, "从新URL下载主题失败: "+err.Error())
					return
				}
				// downloadURL = req.URL
			}
		}
	}

	// 如果没有成功下载主题数据
	if themeData == nil || len(themeData) == 0 {
		api.RespondError(c, http.StatusBadRequest, "无法下载主题，请提供有效的URL或GitHub仓库信息")
		return
	}

	// 到这里，我们已经成功获取了主题数据，可能是通过以下四种方式之一：
	// 1. 原始URL直接下载
	// 2. 原始URL是GitHub仓库，自动获取最新release下载
	// 3. 用户提供的新URL下载
	// 4. 用户提供的GitHub仓库信息，获取最新release下载

	// 临时文件名（随机名，避免并发请求互相覆盖/竞争固定路径）
	tempFile, err := os.CreateTemp("", "komari-theme-download-*.zip")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFile.Name())
	if err := tempFile.Close(); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	if err := os.WriteFile(tempFile.Name(), themeData, 0644); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}

	// 解压ZIP文件并验证
	updatedThemeInfo, err := extractAndValidateTheme(tempFile.Name())
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 如果下载URL与原始URL不同，更新主题配置中的URL
	// if downloadURL != themeInfo.URL {
	// 	updatedThemeInfo.URL = downloadURL

	// 	// 更新主题配置文件
	// 	updatedConfigPath := filepath.Join("./data/theme", updatedThemeInfo.Short, "komari-theme.json")
	// 	updatedConfigData, err := json.MarshalIndent(updatedThemeInfo, "", "  ")
	// 	if err != nil {
	// 		api.RespondError(c, http.StatusInternalServerError, "生成主题配置失败: "+err.Error())
	// 		return
	// 	}

	// 	if err := os.WriteFile(updatedConfigPath, updatedConfigData, 0644); err != nil {
	// 		api.RespondError(c, http.StatusInternalServerError, "更新主题配置文件失败: "+err.Error())
	// 		return
	// 	}
	// }

	api.RespondSuccessMessage(c, "主题更新成功", updatedThemeInfo)
}

// peekThemeFromZip 仅从ZIP文件中读取komari-theme.json并解析主题信息
// 不执行解压安装，用于preview模式
func peekThemeFromZip(zipPath string) (models.Theme, error) {
	var themeInfo models.Theme

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return themeInfo, fmt.Errorf("无法打开ZIP文件: %v", err)
	}
	defer r.Close()

	if err := validateThemeArchive(r.File); err != nil {
		return themeInfo, err
	}

	var themeConfigFile *zip.File
	for _, f := range r.File {
		if f.Name == "komari-theme.json" {
			themeConfigFile = f
			break
		}
	}

	if themeConfigFile == nil {
		return themeInfo, fmt.Errorf("主题配置文件 komari-theme.json 不存在，不是合法的主题包")
	}

	rc, err := themeConfigFile.Open()
	if err != nil {
		return themeInfo, fmt.Errorf("无法读取主题配置文件: %v", err)
	}
	defer rc.Close()

	configData, err := io.ReadAll(io.LimitReader(rc, maxThemeManifestSize+1))
	if err != nil {
		return themeInfo, fmt.Errorf("读取主题配置失败: %v", err)
	}
	if len(configData) > maxThemeManifestSize {
		return themeInfo, fmt.Errorf("主题配置文件超过 %d 字节限制", maxThemeManifestSize)
	}

	if err := json.Unmarshal(configData, &themeInfo); err != nil {
		return themeInfo, fmt.Errorf("主题配置格式错误: %v", err)
	}

	if err := validateThemeManifest(themeInfo); err != nil {
		return themeInfo, err
	}

	return themeInfo, nil
}

// ImportTheme 导入远程主题
// 支持preview查询参数：preview=true时仅返回主题信息，否则下载安装
// 请求body: {"url": "https://..."}
// URL支持GitHub仓库地址（自动取latest release）和直接ZIP下载链接
func ImportTheme(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 解析下载链接
	downloadURL := req.URL
	isGitHub, owner, repo := isGitHubRepoURL(req.URL)
	if isGitHub {
		gitHubURL, err := getGitHubReleaseDownloadURL(owner, repo)
		if err != nil {
			api.RespondError(c, http.StatusBadRequest, "从GitHub获取下载链接失败: "+err.Error())
			return
		}
		downloadURL = gitHubURL
	}

	// 下载主题ZIP
	themeData, err := downloadThemeFromURL(downloadURL)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "下载主题失败: "+err.Error())
		return
	}

	// 保存到临时文件（随机名，避免并发请求互相覆盖/竞争固定路径）
	tempFile, err := os.CreateTemp("", "komari-theme-import-*.zip")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFile.Name())
	if err := tempFile.Close(); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}
	if err := os.WriteFile(tempFile.Name(), themeData, 0644); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存文件失败: "+err.Error())
		return
	}

	// preview模式：仅解析并返回主题信息
	preview := c.Query("preview")
	if preview == "true" {
		themeInfo, err := peekThemeFromZip(tempFile.Name())
		if err != nil {
			api.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}

		// 检查是否已存在同名主题
		exists := false
		themeDir := filepath.Join("./data/theme", themeInfo.Short)
		if _, err := os.Stat(themeDir); err == nil {
			exists = true
		}

		api.RespondSuccess(c, gin.H{
			"theme":  themeInfo,
			"exists": exists,
		})
		return
	}

	// 安装模式：检查是否存在同名主题
	// 先peek一下获取short名称用于检测冲突
	themeInfo, err := peekThemeFromZip(tempFile.Name())
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	overwritten := false
	themeDir := filepath.Join("./data/theme", themeInfo.Short)
	if _, err := os.Stat(themeDir); err == nil {
		overwritten = true
	}

	// 解压安装
	installedTheme, err := extractAndValidateTheme(tempFile.Name())
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	msg := "主题导入成功"
	if overwritten {
		msg = "主题导入成功（已覆盖同名主题）"
	}

	api.RespondSuccessMessage(c, msg, installedTheme)
}

// isInstalledTheme 判断主题是否已安装（嵌入式默认主题或 data/theme 下的目录）。
func isInstalledTheme(short string) bool {
	if !isValidMarketShort(short) {
		return false
	}
	if short == public.DefaultTheme {
		// 默认主题：本地覆盖或嵌入式基线任一存在即可。
		if _, err := os.Stat(filepath.Join("./data/theme", short)); err == nil {
			return true
		}
		_, err := public.PublicFS.ReadFile("defaultTheme/komari-theme.json")
		return err == nil
	}
	_, err := os.Stat(filepath.Join("./data/theme", short))
	return err == nil
}

func UpdateThemeSettings(c *gin.Context) {
	theme := c.Query("theme")
	if theme == "" {
		api.RespondError(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}
	// 校验主题已安装：防止为不存在的主题写入配置记录。
	if !isInstalledTheme(theme) {
		api.RespondError(c, http.StatusNotFound, "主题不存在或未安装")
		return
	}
	var req map[string]any

	err := c.ShouldBindJSON(&req)
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	db := dbcore.GetDBInstance()

	data, err := json.Marshal(&req)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "生成主题配置失败: "+err.Error())
		return
	}

	var themeCfg models.ThemeConfiguration
	db.Where("short = ?", theme).
		Assign(models.ThemeConfiguration{Short: theme, Data: string(data)}).
		FirstOrCreate(&themeCfg)
	api.RespondSuccess(c, nil)
}
