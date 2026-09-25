package main

import (
	"log/slog"

	"github.com/Tumb1er1376/komari-monitor-lite/cmd"
	"github.com/Tumb1er1376/komari-monitor-lite/utils"
	logger "github.com/Tumb1er1376/komari-monitor-lite/utils/log"
)

func main() {
	if utils.VersionHash == "unknown" {
		logger.Setup(slog.LevelDebug)
	} else {
		logger.Setup(slog.LevelInfo)
	}

	logger.Infof("server", "Komari Monitor %s (hash: %s)", utils.CurrentVersion, utils.VersionHash)

	cmd.Execute()
}
