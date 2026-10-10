package main

import (
	"log/slog"

	"github.com/lyhbdw/komari-lite/cmd"
	"github.com/lyhbdw/komari-lite/utils"
	logger "github.com/lyhbdw/komari-lite/utils/log"
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
