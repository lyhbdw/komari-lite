package metric

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaintenanceAction identifies the backend-specific operation used to reclaim
// physical database space.
//
// MaintenanceAction 表示回收数据库物理空间时使用的后端专用操作。
type MaintenanceAction string

const (
	// MaintenanceVacuum is SQLite's VACUUM operation.
	MaintenanceVacuum MaintenanceAction = "vacuum"
	// MaintenanceOptimize is MySQL's OPTIMIZE TABLE operation.
	MaintenanceOptimize MaintenanceAction = "optimize"
	// MaintenanceVacuumFull is PostgreSQL's blocking VACUUM FULL operation.
	MaintenanceVacuumFull MaintenanceAction = "vacuum_full"
)

const (
	sqliteCheckpointSQL = "PRAGMA wal_checkpoint(TRUNCATE)"
	sqliteVacuumSQL     = "VACUUM"
)

// Driver returns the Store's configured database backend.
//
// Driver 返回 Store 配置的数据库后端。
func (s *Store) Driver() Driver {
	return s.cfg.Driver
}

// MaintenanceAction returns the physical-space reclamation operation used by
// the Store's backend.
//
// MaintenanceAction 返回当前后端用于回收物理空间的操作。
func (s *Store) MaintenanceAction() MaintenanceAction {
	return maintenanceActionFor(s.cfg.Driver)
}

// StorageSize returns the physical bytes occupied by this Store. SQLite
// includes the main database, WAL, and shared-memory files. Server backends
// include only the definition, dictionary, and rollup tables managed by Store.
//
// StorageSize 返回当前 Store 占用的物理字节数。SQLite 会统计主数据库、WAL
// 和共享内存文件；服务端数据库只统计 Store 管理的定义、字典和 rollup 表。
func (s *Store) StorageSize(ctx context.Context) (int64, error) {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.db == nil {
		return 0, ErrClosed
	}
	if s.cfg.Driver != DriverSQLite {
		return 0, fmt.Errorf("%w: storage size requires SQLite", ErrInvalidArgument)
	}
	return s.sqliteStorageSize(ctx)
}

// SnapshotTo writes a consistent SQLite snapshot to destPath. SQLite reads
// committed pages from the live database and includes WAL content in the
// snapshot, so callers never copy an active database file by itself.
func (s *Store) SnapshotTo(ctx context.Context, destPath string) error {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.db == nil {
		return ErrClosed
	}
	if s.cfg.Driver != DriverSQLite {
		return fmt.Errorf("%w: snapshots require SQLite", ErrInvalidArgument)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("metric: create snapshot directory: %w", err)
	}
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("metric: remove existing snapshot: %w", err)
	}
	safePath := strings.ReplaceAll(filepath.ToSlash(destPath), "'", "''")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+safePath+"'"); err != nil {
		return fmt.Errorf("metric: snapshot sqlite database: %w", err)
	}
	return nil
}

// ReclaimSpace performs the backend-specific blocking operation that returns
// unused database pages to the filesystem. It serializes against other
// maintenance calls and keeps Close from closing the pool mid-operation.
//
// ReclaimSpace 执行后端专用的阻塞式空间回收操作。该方法会与其他维护调用
// 串行执行，并阻止 Close 在操作过程中关闭连接池。
func (s *Store) ReclaimSpace(_ context.Context) error {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	// Space reclamation is an explicit, non-cancellable operation. In
	// particular VACUUM cannot be resumed safely from a short request deadline;
	// once admitted, it runs to completion or returns a database error.
	ctx := context.Background()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.db == nil {
		return ErrClosed
	}
	if _, err := s.cleanupOrphanedMetricData(ctx); err != nil {
		return err
	}

	switch s.cfg.Driver {
	case DriverSQLite:
		if err := sqliteCheckpoint(ctx, s.db); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, sqliteVacuumSQL); err != nil {
			return fmt.Errorf("metric: vacuum sqlite database: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, "PRAGMA optimize"); err != nil {
			return fmt.Errorf("metric: optimize sqlite database: %w", err)
		}
		// VACUUM itself can populate the WAL; truncate it again so the reported
		// physical size reflects the completed reclamation.
		return sqliteCheckpoint(ctx, s.db)
	default:
		return fmt.Errorf("%w: unsupported driver %q", ErrInvalidArgument, s.cfg.Driver)
	}
}

// cleanupOrphanedMetricData removes rows whose metric definition no longer
// exists. It runs immediately before an explicit space-reclaim operation so
// the physical rewrite returns the freed pages to the filesystem in the same
// maintenance window.
func (s *Store) cleanupOrphanedMetricData(ctx context.Context) (int64, error) {
	var deleted int64
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM %s WHERE NOT EXISTS (SELECT 1 FROM %s s JOIN %s d ON d.name = s.metric_name WHERE s.id = %s.series_id)`,
		s.tables.rollups, s.tables.series, s.tables.definitions, s.tables.rollups))
	if err != nil {
		return deleted, fmt.Errorf("metric: delete orphaned rows from %s: %w", s.tables.rollups, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return deleted, err
	}
	deleted += count
	for _, statement := range []string{
		fmt.Sprintf(`DELETE FROM %s WHERE NOT EXISTS (SELECT 1 FROM %s r WHERE r.series_id = %s.id)`, s.tables.series, s.tables.rollups, s.tables.series),
		fmt.Sprintf(`DELETE FROM %s WHERE NOT EXISTS (SELECT 1 FROM %s r WHERE r.label_id = %s.id)`, s.tables.labels, s.tables.rollups, s.tables.labels),
		fmt.Sprintf(`DELETE FROM %s WHERE NOT EXISTS (SELECT 1 FROM %s r WHERE r.resolution_id = %s.id)`, s.tables.resolutions, s.tables.rollups, s.tables.resolutions),
	} {
		result, err := s.db.ExecContext(ctx, statement)
		if err != nil {
			return deleted, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += count
	}
	return deleted, nil
}

func (s *Store) sqliteStorageSize(ctx context.Context) (int64, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return 0, fmt.Errorf("metric: list sqlite databases: %w", err)
	}

	var path string
	for rows.Next() {
		var sequence int
		var name, file string
		if err := rows.Scan(&sequence, &name, &file); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("metric: scan sqlite database path: %w", err)
		}
		if name == "main" {
			path = file
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("metric: list sqlite databases: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("metric: close sqlite database list: %w", err)
	}
	if path == "" {
		// SQLite reports an empty path for in-memory databases.
		return 0, nil
	}

	var size int64
	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(name)
		switch {
		case err == nil:
			size += info.Size()
		case errors.Is(err, os.ErrNotExist):
		default:
			return 0, fmt.Errorf("metric: stat sqlite storage file %q: %w", name, err)
		}
	}
	return size, nil
}

func sqliteCheckpoint(ctx context.Context, db *sql.DB) error {
	var busy, logFrames, checkpointedFrames int
	if err := db.QueryRowContext(ctx, sqliteCheckpointSQL).Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return fmt.Errorf("metric: checkpoint sqlite WAL: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("metric: checkpoint sqlite WAL: database is busy (%d log frames, %d checkpointed)", logFrames, checkpointedFrames)
	}
	return nil
}

func maintenanceActionFor(driver Driver) MaintenanceAction {
	if driver == DriverSQLite {
		return MaintenanceVacuum
	}
	return ""
}


