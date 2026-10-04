package metric

import (
	"testing"
)

// TestBackendBuildersApplyOptions verifies backend builders apply options.
//
// TestBackendBuildersApplyOptions 验证各后端构造器会正确应用选项。
func TestBackendBuildersApplyOptions(t *testing.T) {
	cfg := SQLite(
		"file:metrics.db?cache=shared",
		WithTablePrefix("x_metric_"),
		WithAutoMigrate(false),
		WithMaxOpenConns(10),
		WithMaxIdleConns(2),
	)

	if cfg.Driver != DriverSQLite {
		t.Fatalf("expected sqlite driver, got %q", cfg.Driver)
	}
	if cfg.TablePrefix != "x_metric_" {
		t.Fatalf("table prefix option was not applied: %q", cfg.TablePrefix)
	}
	if cfg.AutoMigrate {
		t.Fatalf("auto migrate option was not applied")
	}
	if cfg.MaxOpenConns != 10 || cfg.MaxIdleConns != 2 {
		t.Fatalf("pool options were not applied: %#v", cfg)
	}
	if cfg.ConnMaxLifetime != 0 {
		t.Fatalf("sqlite default should keep connections until close: %v", cfg.ConnMaxLifetime)
	}
}
