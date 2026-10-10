package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/internal/managedconfig"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	logger "github.com/lyhbdw/komari-lite/utils/log"
	"github.com/lyhbdw/komari-lite/web/public"
)

// publicInfoQueryTimeout 限制访客信息聚合查询的后台耗时，防止慢查询
// 无限期挂起请求 goroutine。
const publicInfoQueryTimeout = 30 * time.Second

func GetPublicInfo() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), publicInfoQueryTimeout)
	defer cancel()

	cstPtr, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return nil, err
	}
	cst := *cstPtr

	all, allErr := config.GetAll()
	hasKey := func(k string) bool {
		if allErr != nil {
			return false
		}
		_, ok := all[k]
		return ok
	}

	// Apply defaults only when a key is missing.
	if !hasKey("sitename") {
		cst.Sitename = "Komari"
	}
	if !hasKey("description") {
		cst.Description = "Komari Monitor, a simple server monitoring tool."
	}
	cst.Theme = public.DefaultTheme

	// Fallback defaults if we couldn't enumerate keys.
	if allErr != nil {
		if cst.Sitename == "" {
			cst.Sitename = "Komari"
		}
		if cst.Description == "" {
			cst.Description = "Komari Monitor, a simple server monitoring tool."
		}
	}
	retention, err := metricstore.GetRetentionSummary(ctx)
	if err != nil {
		return nil, err
	}
	db := dbcore.GetDBInstance()
	tc := models.ThemeConfiguration{}
	err = db.Model(&models.ThemeConfiguration{}).Where("short = ?", cst.Theme).First(&tc).Error
	if err != nil {
		tc.Data = "{}"
	}
	tc_data := gin.H{}
	err = json.Unmarshal([]byte(tc.Data), &tc_data)
	if err != nil {
		logger.Infof("database", "%v", err)
	}
	items := themeConfigurationItems(cst.Theme)
	if cst.Theme != public.DefaultTheme {
		for _, item := range items {
			if item.Key == "" {
				continue
			}
			if _, exists := tc_data[item.Key]; !exists {
				tc_data[item.Key] = managedconfig.DefaultValue(item)
			}
		}
	}
	if err := managedconfig.ResolveForOutput(tc_data, items); err != nil {
		return nil, err
	}

	return gin.H{
		"sitename":    cst.Sitename,
		"description": cst.Description,

		"disable_password_login":    cst.DisablePasswordLogin,
		"cors_origin_check_enabled": true,
		"record_enabled":            true,
		"record_preserve_time":      retention.MaxDays * 24,
		"ping_record_preserve_time": retention.MaxDays * 24,
		"theme":                     cst.Theme,
		"theme_settings":            tc_data,
	}, nil
}

var (
	defaultThemeItemsCache []models.ManagedThemeConfigurationItem
	defaultThemeItemsOnce  sync.Once
)

func themeConfigurationItems(short string) []models.ManagedThemeConfigurationItem {
	var manifest models.Theme
	if short == public.DefaultTheme {
		defaultThemeItemsOnce.Do(func() {
			data, err := public.PublicFS.ReadFile("defaultTheme/komari-theme.json")
			if err == nil && json.Unmarshal(data, &manifest) == nil {
				defaultThemeItemsCache = managedconfig.Items(manifest.Configuration)
			}
		})
		return defaultThemeItemsCache
	}
	data, err := os.ReadFile(filepath.Join("./data/theme", short, "komari-theme.json"))
	if err != nil || json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	return managedconfig.Items(manifest.Configuration)
}
