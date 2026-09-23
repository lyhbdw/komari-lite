package metric

import "testing"

// TestDialectsGenerateBackendSpecificSQL verifies SQLite dialect rendering.
//
// TestDialectsGenerateBackendSpecificSQL 验证 SQLite 方言渲染。
// Lite 只支持 SQLite，此测试同时锁定 newDialect 对任何 driver 输入都返回
// sqlite 行为（防止未来重新引入多后端时静默回归）。
func TestDialectsGenerateBackendSpecificSQL(t *testing.T) {
	for _, driver := range []Driver{DriverSQLite} {
		t.Run(string(driver), func(t *testing.T) {
			d := newDialect(driver)
			if got := d.placeholder(1); got != "?" {
				t.Fatalf("placeholder: expected %q, got %q", "?", got)
			}
			if got := d.jsonType(); got != "TEXT" {
				t.Fatalf("json type: expected %q, got %q", "TEXT", got)
			}
			if got := d.blobType(); got != "BLOB" {
				t.Fatalf("blob type: expected %q, got %q", "BLOB", got)
			}
			if got := d.jsonPlaceholder(1); got != "?" {
				t.Fatalf("json placeholder: expected %q, got %q", "?", got)
			}
			if got := d.autoIncrementPrimaryKey(); got != "INTEGER PRIMARY KEY AUTOINCREMENT" {
				t.Fatalf("auto increment: unexpected %q", got)
			}
		})
	}
}
