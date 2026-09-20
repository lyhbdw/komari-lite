package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/public"
)

// ListThemes lists the embedded default theme and locally installed themes.
func ListThemes(c *gin.Context) {
	dataDir := "./data/theme"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		api.RespondSuccess(c, []models.Theme{})
		return
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "读取主题目录失败: "+err.Error())
		return
	}

	themes := make([]models.Theme, 0, len(entries)+1)
	if defaultTheme, err := public.PublicFS.ReadFile("defaultTheme/komari-theme.json"); err == nil {
		var theme models.Theme
		if json.Unmarshal(defaultTheme, &theme) == nil {
			themes = append(themes, theme)
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if theme, err := loadThemeConfig(filepath.Join(dataDir, entry.Name(), "komari-theme.json")); err == nil {
			themes = append(themes, theme)
		}
	}
	api.RespondSuccess(c, themes)
}

// SetTheme selects the embedded default theme or an already installed theme.
func SetTheme(c *gin.Context) {
	themeName := c.Query("theme")
	if themeName == "" {
		api.RespondError(c, http.StatusBadRequest, "主题名称不能为空")
		return
	}
	if themeName != "default" {
		if !isValidThemeName(themeName) {
			api.RespondError(c, http.StatusBadRequest, "无效的主题名称")
			return
		}
		configPath := filepath.Join("./data/theme", themeName, "komari-theme.json")
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				api.RespondError(c, http.StatusNotFound, "主题不存在")
			} else {
				api.RespondError(c, http.StatusInternalServerError, "读取主题失败: "+err.Error())
			}
			return
		}
	}
	if err := config.Set(config.ThemeKey, themeName); err != nil {
		api.RespondError(c, http.StatusInternalServerError, "更新主题设置失败: "+err.Error())
		return
	}
	api.RespondSuccessMessage(c, "主题设置成功", gin.H{"theme": themeName})
}

func UpdateThemeSettings(c *gin.Context) {
	theme := c.Query("theme")
	if !isValidThemeName(theme) {
		api.RespondError(c, http.StatusBadRequest, "主题名称不能为空或格式无效")
		return
	}
	var values map[string]any
	if err := c.ShouldBindJSON(&values); err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	data, err := json.Marshal(values)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "生成主题配置失败: "+err.Error())
		return
	}
	var themeConfig models.ThemeConfiguration
	if err := dbcore.GetDBInstance().Where("short = ?", theme).
		Assign(models.ThemeConfiguration{Short: theme, Data: string(data)}).
		FirstOrCreate(&themeConfig).Error; err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存主题配置失败: "+err.Error())
		return
	}
	api.RespondSuccess(c, nil)
}

func loadThemeConfig(path string) (models.Theme, error) {
	var theme models.Theme
	data, err := os.ReadFile(path)
	if err != nil {
		return theme, err
	}
	if err := json.Unmarshal(data, &theme); err != nil {
		return theme, err
	}
	if !isValidThemeName(theme.Short) {
		return models.Theme{}, os.ErrInvalid
	}
	return theme, nil
}

func isValidThemeName(name string) bool {
	if name == "" || name == "default" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return !strings.Contains(name, "..")
}
