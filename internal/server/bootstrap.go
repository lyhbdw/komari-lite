package server

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/Tumb1er1376/komari-monitor-lite/database/dbcore"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/config"
	"github.com/Tumb1er1376/komari-monitor-lite/utils"
)

// Bootstrap initializes the data directory, primary database, and settings.
func (a *App) Bootstrap() error {
	if err := os.MkdirAll("./data/theme", os.ModePerm); err != nil {
		return fmt.Errorf("failed to create theme directory: %w", err)
	}

	dbcore.SetVersionID(utils.CurrentVersion + "-" + utils.VersionHash)
	if err := dbcore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	a.dbReady = true
	a.addCleanup("database", func(context.Context) error { return dbcore.Close() })

	gin.SetMode(gin.ReleaseMode)
	settings, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	a.settings = settings
	return nil
}
