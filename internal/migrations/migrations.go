package migrations

import (
	"context"

	"gorm.io/gorm"
)

// Context carries the startup database connection.
type Context struct {
	DB *gorm.DB
}

// Run executes the startup upgrade path.
//
// 历史一次性迁移（legacy configs client_infos/timestamp UTC 等）已随
// 早期版本完成，不再保留；旧 monitoring 表导入改为按需触发：
// 管理员调用 admin.migration RPC，入口是 RunLegacyMonitoringIfRequired。
func Run(ctx context.Context, dbCtx Context) error {
	db := dbCtx.DB
	if db == nil {
		return nil
	}
	_, err := RunLegacyMonitoringIfRequired(ctx, db)
	return err
}

// RunLegacyMonitoringIfRequired imports legacy monitoring rows when the
// marker table still exists. Runs at most once per marker state and is safe
// to call concurrently with the admin RPC.
func RunLegacyMonitoringIfRequired(ctx context.Context, db *gorm.DB) (bool, error) {
	if db == nil {
		return false, nil
	}
	required, _, err := LegacyMonitoringMigrationRequired(db)
	if err != nil || !required {
		return false, err
	}
	return true, CompleteLegacyMonitoringMigration(db, nil)
}
