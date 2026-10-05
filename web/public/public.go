package public

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-monitor-lite/internal/config"
)

//go:embed defaultTheme/komari-theme.json
var PublicFS embed.FS

//go:embed agent-install.sh
var AgentInstallScript []byte

//go:embed defaultTheme/dist.tar.zst
var embeddedDistArchive []byte

// 常量定义
const (
	DataDir            = "./data"
	ThemesDir          = "theme"
	FaviconFile        = "favicon.ico"
	DefaultTheme       = "Lite"
	LanguageCookieName = "language"

	// 主题内部结构定义
	DistDir   = "dist"       // 静态资源存放目录
	IndexFile = "index.html" // 相对于 DistDir
)

//go:embed defaultTheme/admin-dist.tar.zst
var embeddedAdminDistArchive []byte

func init() {
	_ = os.MkdirAll("./data/theme", 0755)

	var err error
	defaultDistFiles, err = loadEmbeddedDist()
	if err != nil {
		panic("load embedded default frontend: " + err.Error())
	}
	adminDistFiles, err = decodeEmbeddedDist(embeddedAdminDistArchive)
	if err != nil {
		panic("load embedded admin frontend: " + err.Error())
	}
}

func normalizeHTMLLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "_", "-"))
	if len(language) < 2 || len(language) > 32 {
		return ""
	}

	for _, r := range language {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return ""
	}

	return language
}

func replaceHTMLLanguage(htmlStr, language string) string {
	language = normalizeHTMLLanguage(language)
	if language == "" {
		return htmlStr
	}

	replacements := []struct {
		old string
		new string
	}{
		{`<html lang="en">`, `<html lang="` + language + `">`},
		{`<html lang='en'>`, `<html lang='` + language + `'>`},
		{`<html>`, `<html lang="` + language + `">`},
	}

	for _, replacement := range replacements {
		if strings.Contains(htmlStr, replacement.old) {
			return strings.Replace(htmlStr, replacement.old, replacement.new, 1)
		}
	}

	return htmlStr
}

func stripServiceWorkerRegistration(html string) string {
	return strings.ReplaceAll(html, `<script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script>`, "")
}

func adminAssetRequest(r *http.Request) bool {
	referrer := r.Referer()
	if referrer == "" {
		return false
	}
	u, err := url.Parse(referrer)
	if err != nil {
		return false
	}
	return strings.HasPrefix(u.Path, "/admin") || strings.HasPrefix(u.Path, "/terminal")
}

// isSafePath 验证路径是否在指定的基础目录内，防止路径穿透攻击
func isSafePath(basePath, targetPath string) bool {
	// 获取基础目录的绝对路径
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}

	// 清理目标路径，移除 ../ 等
	cleanTarget := filepath.Clean(targetPath)

	// 拼接完整路径
	fullPath := filepath.Join(absBase, cleanTarget)

	// 获取绝对路径
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}

	// 检查目标路径是否以基础路径开头
	// 使用 filepath.Rel 更可靠地检查路径关系
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}

	// 如果相对路径以 .. 开头，说明目标在基础目录之外
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// Static 注册静态资源和 SPA 路由处理
func Static(r *gin.RouterGroup, noRoute func(handlers ...gin.HandlerFunc)) {
	static(r, noRoute, false)
}

func static(r *gin.RouterGroup, noRoute func(handlers ...gin.HandlerFunc), forceDefaultTheme bool) {
	// 初始化嵌入式文件系统，指向 defaultTheme 根目录。
	defaultThemeFS, err := fs.Sub(PublicFS, "defaultTheme")
	if err != nil {
		panic("embedded default theme metadata is unavailable: " + err.Error())
	}

	getConfig := func() map[string]any {
		cfg, _ := config.GetMany(map[string]any{
			config.DescriptionKey: "A simple server monitor tool.",

			config.SitenameKey: "Komari Monitor",
			config.ThemeKey:    DefaultTheme,
		})
		cfg[config.ThemeKey] = DefaultTheme
		return cfg
	}

	// 核心逻辑：获取文件内容
	// filePath: 相对于主题根目录的路径 (例如 "theme.json" 或 "dist/assets/a.js")
	// 返回: content, contentType, exists
	getFileContent := func(themeID string, relativePath string) ([]byte, string, bool) {
		cleanPath := strings.TrimPrefix(relativePath, "/")

		cleanPath = filepath.Clean(cleanPath)
		embedPath := filepath.ToSlash(cleanPath)

		if themeID == "__admin__" {
			if content, ok := adminDistFiles[strings.TrimPrefix(embedPath, DistDir+"/")]; ok {
				return content, mime.TypeByExtension(filepath.Ext(embedPath)), true
			}
			return nil, "", false
		}

		if strings.Contains(themeID, "..") || strings.Contains(themeID, "/") || strings.Contains(themeID, "\\") {
			return nil, "", false
		}
		themeBasePath := filepath.Join(DataDir, ThemesDir, themeID)
		if !isSafePath(themeBasePath, cleanPath) {
			return nil, "", false
		}
		localPath := filepath.Join(themeBasePath, cleanPath)
		if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
			content, err := os.ReadFile(localPath)
			if err == nil {
				return content, mime.TypeByExtension(filepath.Ext(localPath)), true
			}
		}
		if themeID != DefaultTheme {
			return nil, "", false
		}

		// 2. 尝试从嵌入式 defaultTheme/{cleanPath} 读取
		// fs.ReadFile 处理 embed 路径时使用 "/"
		if strings.Contains(embedPath, "..") {
			return nil, "", false
		}

		if strings.HasPrefix(embedPath, DistDir+"/") {
			if content, ok := defaultDistFiles[strings.TrimPrefix(embedPath, DistDir+"/")]; ok {
				return content, mime.TypeByExtension(filepath.Ext(embedPath)), true
			}
		} else if content, err := fs.ReadFile(defaultThemeFS, embedPath); err == nil {
			return content, mime.TypeByExtension(filepath.Ext(embedPath)), true
		}

		return nil, "", false
	}

	// 核心逻辑：渲染 Index.html
	serveIndex := func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		cfg := getConfig()

		currentTheme := cfg[config.ThemeKey].(string)
		shouldReplace := true

		// 特殊页面：强制使用 default 主题，且不进行内容替换
		if forceDefaultTheme || strings.HasPrefix(reqPath, "/admin") || strings.HasPrefix(reqPath, "/terminal") || adminAssetRequest(c.Request) {
			currentTheme = "__admin__"
			shouldReplace = false
		}

		// 获取 dist/index.html (相对于主题根目录)
		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := getFileContent(currentTheme, targetFile)

		if !exists {
			c.String(http.StatusNotFound, "Index file missing (checked %s/dist/index.html and default).", currentTheme)
			return
		}

		htmlStr := string(content)
		if forceDefaultTheme {
			htmlStr = stripServiceWorkerRegistration(htmlStr)
		}
		if language, err := c.Cookie(LanguageCookieName); err == nil {
			htmlStr = replaceHTMLLanguage(htmlStr, language)
		}

		// 如果不替换，保留系统内置页面内容，仅同步 html lang。
		if !shouldReplace {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlStr))
			return
		}

		// 执行 HTML 内容替换
		sitename := cfg[config.SitenameKey].(string)
		replacer := strings.NewReplacer(
			"<title>Komari Monitor</title>", "<title>"+sitename+"</title>",
			"<title>Monitor</title>", "<title>"+sitename+"</title>",
			"A simple server monitor tool.", cfg[config.DescriptionKey].(string),
			"<head>", "<head><script>window.__INITIAL_SITENAME__="+strconv.Quote(sitename)+";</script>",
		)

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(replacer.Replace(htmlStr)))
	}

	// ================= 路由定义 =================
	// 1. Favicon 优先策略
	serveFavicon := func(c *gin.Context) {
		// 优先：./data/favicon.ico
		localFavicon := filepath.Join(DataDir, FaviconFile)
		if !forceDefaultTheme {
			if _, err := os.Stat(localFavicon); err == nil {
				c.File(localFavicon)
				return
			}
		}

		// 其次：当前主题的 dist/favicon.ico 或 theme_root/favicon.ico ?
		cfg := getConfig()
		themeFaviconPath := path.Join(DistDir, FaviconFile)
		currentTheme := cfg[config.ThemeKey].(string)
		if forceDefaultTheme || strings.HasPrefix(c.Request.URL.Path, "/admin") || strings.HasPrefix(c.Request.URL.Path, "/terminal") {
			currentTheme = "__admin__"
		}
		content, mimeType, exists := getFileContent(currentTheme, themeFaviconPath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}

		c.Status(http.StatusNotFound)
	}
	r.GET("/favicon.ico", serveFavicon)
	r.HEAD("/favicon.ico", serveFavicon)

	serveStaticIcon := func(filename, mimeType string) gin.HandlerFunc {
		return func(c *gin.Context) {
			localFile := filepath.Join(DataDir, filename)
			if _, err := os.Stat(localFile); err == nil {
				c.File(localFile)
				return
			}
			content, _, exists := getFileContent("__admin__", path.Join(DistDir, filename))
			if exists {
				c.Data(http.StatusOK, mimeType, content)
				return
			}
			c.Status(http.StatusNotFound)
		}
	}
	r.GET("/favicon.svg", serveStaticIcon("favicon.svg", "image/svg+xml"))
	r.GET("/apple-touch-icon.png", serveStaticIcon("apple-touch-icon.png", "image/png"))

	// 2. 静态资源路由 /themes/:id/*path
	// 允许访问 /themes/MyTheme/theme.json 和 /themes/MyTheme/dist/assets/a.js
	r.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		if themeID != DefaultTheme && themeID != "__admin__" {
			c.Status(http.StatusNotFound)
			return
		}
		if forceDefaultTheme && themeID != "__admin__" && themeID != DefaultTheme {
			c.Status(http.StatusNotFound)
			return
		}
		if forceDefaultTheme {
			themeID = "__admin__"
		}
		// c.Param("path") 包含了开头的 /，getFileContent 会处理
		filePath := c.Param("path")

		content, mimeType, exists := getFileContent(themeID, filePath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// 3. SPA 路由 (noRoute)
	noRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}
		reqPath := c.Request.URL.Path
		cfg := getConfig()
		currentTheme := cfg[config.ThemeKey].(string)
		if forceDefaultTheme || strings.HasPrefix(reqPath, "/admin") || strings.HasPrefix(reqPath, "/terminal") || adminAssetRequest(c.Request) {
			currentTheme = "__admin__"
		}

		// SPA 静态资源回退
		distPath := path.Join(DistDir, reqPath)
		if strings.HasPrefix(reqPath, "/assets/") {
			if content, mimeType, exists := getFileContent("__admin__", distPath); exists {
				c.Data(http.StatusOK, mimeType, content)
				return
			}
		}

		content, mimeType, exists := getFileContent(currentTheme, distPath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}

		// 路由 (如 /dashboard, /settings) -> 返回 index.html
		serveIndex(c)
	})
}
