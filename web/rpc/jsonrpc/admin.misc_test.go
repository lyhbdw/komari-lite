package jsonrpc

import "testing"

func TestRemoveRetiredSettings(t *testing.T) {
	cfg := map[string]interface{}{
		"low_resource_mode":                           true,
		"theme":                                       "OtherTheme",
		"metric_db_driver":                            "sqlite",
		"metric_db_dsn":                               "./data/metrics.db",
		"metric_table_prefix":                         "metric_",
		"metric_max_open_conns":                       25,
		"metric_max_idle_conns":                       5,
		"metric_rollup_minute_retention_minutes":      600,
		"metric_rollup_five_minute_retention_minutes": 3000,
		"metric_rollup_hour_retention_hours":          600,
		"metric_migration_target":                     "sqlite|./data/metrics.db",
		"sitename":                                    "Komari",
	}

	removeRetiredLowResourceMode(cfg)
	enforceLiteThemeSettings(cfg)
	removeRetiredMetricStoreConfig(cfg)

	for _, key := range []string{
		"low_resource_mode",
		"metric_db_driver",
		"metric_db_dsn",
		"metric_table_prefix",
		"metric_max_open_conns",
		"metric_max_idle_conns",
		"metric_rollup_minute_retention_minutes",
		"metric_rollup_five_minute_retention_minutes",
		"metric_rollup_hour_retention_hours",
		"metric_migration_target",
	} {
		if _, ok := cfg[key]; ok {
			t.Fatalf("retired setting %q must not be persisted", key)
		}
	}
	if cfg["sitename"] != "Komari" {
		t.Fatal("unrelated settings must be preserved")
	}
	if cfg["theme"] != "Emerald" {
		t.Fatal("Lite theme must remain fixed to Emerald")
	}
}
