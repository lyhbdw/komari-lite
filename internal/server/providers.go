package server

import (
	"context"

	"github.com/lyhbdw/komari-monitor-lite/utils/geoip"
	"github.com/lyhbdw/komari-monitor-lite/utils/messageSender"
)

// InitProviders initializes providers used by the monitoring application.
func (a *App) InitProviders() error {
	go geoip.InitGeoIp()
	a.addCleanup("geoip", func(context.Context) error { return geoip.Shutdown() })

	messageSender.Initialize()
	a.addCleanup("message-sender", func(context.Context) error { return messageSender.Shutdown() })
	return nil
}
