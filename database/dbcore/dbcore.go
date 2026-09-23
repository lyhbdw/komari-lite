package dbcore

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	appconfig "github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/migrations"
	"github.com/komari-monitor/komari/internal/sqlitetune"
	logger "github.com/komari-monitor/komari/utils/log"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// zipDirectoryExcluding 将 srcDir 打包为 dstZip，exclude 是绝对路径集合需要排除
//
// 任一步失败时删除半成品 zip，避免留下截断的归档被误当作可用备份。
func zipDirectoryExcluding(srcDir, dstZip string, exclude map[string]struct{}) error {
	// 规范化排除路径为绝对路径
	normExclude := make(map[string]struct{}, len(exclude))
	for p := range exclude {
		abs, _ := filepath.Abs(p)
		normExclude[abs] = struct{}{}
	}

	out, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)

	absSrc, _ := filepath.Abs(srcDir)
	walkErr := filepath.Walk(absSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 排除 backup.zip 本身
		if _, ok := normExclude[path]; ok {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// 计算 zip 内相对路径
		rel, err := filepath.Rel(absSrc, path)
		if err != nil {
			return err
		}
		// 根目录跳过
		if rel == "." {
			return nil
		}
		// 替换为正斜杠
		zipName := filepath.ToSlash(rel)

		if info.IsDir() {
			_, err := zw.Create(zipName + "/")
			return err
		}
		// 普通文件
		fh, err := os.Open(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(zipName)
		if err != nil {
			fh.Close()
			return err
		}
		if _, err := io.Copy(w, fh); err != nil {
			fh.Close()
			return err
		}
		fh.Close()
		return nil
	})
	if walkErr != nil {
		_ = zw.Close()
		_ = out.Close()
		_ = os.Remove(dstZip)
		return walkErr
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(dstZip)
		return err
	}
	return out.Close()
}

var (
	instance *gorm.DB
	once     sync.Once
	initErr  error
)

// SystemVersionKey 是记录“上次启动所用版本标识”的配置键（存于 configs 表）。
// 取代旧的 ./data/.komari-version 文件：版本标识随配置库一起备份/恢复，
// 也避免额外的裸文件依赖。
const SystemVersionKey = "system_version"

const (
	mainSQLiteBusyTimeout       = 5 * time.Second
	mainSQLiteCacheSizeKB       = 8 * 1024
	mainSQLiteWALAutoCheckpoint = 256
	mainSQLiteJournalSizeLimit  = 1 << 20
)

// versionID 是当前构建的版本标识，由 SetVersionID 在 Initialize 前注入。
var versionID string

// dbFileExistedAtStartup 记录本次进程启动、打开数据库之前 komari.db 是否已存在，
// 用于区分“全新安装”与“从旧版升级（无版本标记）”。在 doInitialize 打开数据库
// 之前采集。
var dbFileExistedAtStartup bool

// SetVersionID 设置当前构建的版本标识（通常为 CurrentVersion+"-"+VersionHash），
// 用于版本升级检测与自动备份。应在 Initialize() 之前调用；为空则跳过升级备份。
func SetVersionID(id string) {
	versionID = id
}

// resolveDatabaseFile 返回当前使用的 SQLite 数据库文件路径。
func resolveDatabaseFile() string {
	dbFile := flags.DatabaseFile
	if dbFile == "" {
		dbFile = "./data/komari.db"
	}
	return dbFile
}

// backupOnVersionUpgrade 在检测到版本升级时，把当前 ./data 的一致性快照
// 打包到 ./data/backup/upgrade-{time}.zip，便于升级（含 metrics 迁移）异常时回滚。
//
// 版本标识存放于配置库（configs 表，键 system_version），因此本函数必须在
// config.SetDb 之后、一次性 metrics 迁移（InitStores）之前调用。
//
// 触发规则：
//   - versionID 为空：跳过（未注入版本，如部分测试场景）。
//   - 配置中无版本且启动前无数据库文件：全新安装，仅写版本，不备份。
//   - 配置中无版本但启动前已有数据库文件：从无版本标记的旧稳定版升级，备份。
//   - 配置中版本与当前不同：版本升级，备份。
//   - 配置中版本与当前一致：无需备份。
//
// 备份失败不阻止启动，但打印明确错误；备份成功（或无需备份）后写入/更新版本。
func backupOnVersionUpgrade() {
	if versionID == "" {
		return
	}

	prevVersion, readErr := readSystemVersion()
	prevVersion = strings.TrimSpace(prevVersion)
	versionRecorded := readErr == nil && prevVersion != ""

	// 版本未变化，无需备份。
	if versionRecorded && prevVersion == versionID {
		return
	}

	// 全新安装：配置中无版本且启动前无数据库文件，直接写版本不备份。
	if !versionRecorded && !dbFileExistedAtStartup {
		writeVersionMarker()
		return
	}

	// 备份目录从实际 DB 路径推算（与 createUpgradeBackup 的 dataDir 一致），
	// 避免硬编码 ./data/backup 与自定义 -database 路径不一致。
	backupDir := filepath.Join(filepath.Dir(resolveDatabaseFile()), "backup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to create backup dir: %v", err)
		return
	}
	tsName := time.Now().UTC().Format("20060102-150405")
	bakPath := filepath.Join(backupDir, fmt.Sprintf("upgrade-%s.zip", tsName))
	if err := createUpgradeBackup(bakPath); err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to backup ./data before upgrade (from %q to %q): %v", prevVersion, versionID, err)
		return
	}
	logger.Infof("dbcore", "[upgrade-backup] ./data backed up to %s before upgrade (from %q to %q)", bakPath, prevVersion, versionID)

	pruneUpgradeBackups(backupDir)

	writeVersionMarker()
}

// maxUpgradeBackups 是升级备份的保留份数。每次版本升级都会产生一个
// ~数百 MB 的全量 zip，只增不减会持续吃掉磁盘；保留最近 2 份足够覆盖
// "升级后发现严重问题 → 回滚到上一稳定版"的场景。
const maxUpgradeBackups = 2

// pruneUpgradeBackups 删除 backupDir 中超出保留份数的最旧 upgrade-*.zip。
// 只匹配 upgrade- 前缀 + .zip 后缀，不触碰管理员手动放置的其他归档；
// 任何删除失败只记日志，绝不影响启动。
func pruneUpgradeBackups(backupDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to list backup dir for pruning: %v", err)
		return
	}
	type backupFile struct {
		name string
		mod  time.Time
	}
	var backups []backupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "upgrade-") || !strings.HasSuffix(name, ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupFile{name: name, mod: info.ModTime()})
	}
	if len(backups) <= maxUpgradeBackups {
		return
	}
	// 按修改时间从旧到新排序，删掉最旧的超出部分。
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].mod.Before(backups[j].mod)
	})
	for _, old := range backups[:len(backups)-maxUpgradeBackups] {
		if err := os.Remove(filepath.Join(backupDir, old.name)); err != nil {
			logger.Errorf("dbcore", "[upgrade-backup] failed to remove stale upgrade backup %s: %v", old.name, err)
			continue
		}
		logger.Infof("dbcore", "[upgrade-backup] pruned stale upgrade backup %s", old.name)
	}
}

// createUpgradeBackup archives non-database data together with SQLite-consistent
// snapshots of both databases. Live database files, WAL and SHM sidecars are
// never copied directly.
func createUpgradeBackup(archivePath string) error {
	mainDB := resolveDatabaseFile()
	dataDir := filepath.Dir(mainDB)
	if dataDir == "." || dataDir == "" {
		dataDir = "."
	}
	metricsDB := filepath.Join(dataDir, "metrics.db")
	stagingDir, err := os.MkdirTemp(dataDir, ".upgrade-backup-*")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if err := copyUpgradeData(dataDir, stagingDir, mainDB, metricsDB, filepath.Join(dataDir, "backup"), stagingDir); err != nil {
		return fmt.Errorf("copy non-database data: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("main database is not initialized")
	}
	mainSQLDB, err := instance.DB()
	if err != nil {
		return fmt.Errorf("get main database connection: %w", err)
	}
	if err := snapshotSQLiteConnection(mainSQLDB, filepath.Join(stagingDir, "komari.db")); err != nil {
		return fmt.Errorf("snapshot main database: %w", err)
	}

	if _, err := os.Stat(metricsDB); err == nil {
		metricsSQLDB, err := sql.Open("sqlite3", buildSQLiteDSN(metricsDB))
		if err != nil {
			return fmt.Errorf("open metrics database: %w", err)
		}
		metricsSQLDB.SetMaxOpenConns(1)
		defer metricsSQLDB.Close()
		if err := snapshotSQLiteConnection(metricsSQLDB, filepath.Join(stagingDir, "metrics.db")); err != nil {
			return fmt.Errorf("snapshot metrics database: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat metrics database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}
	if err := zipDirectoryExcluding(stagingDir, archivePath, nil); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	return nil
}

func copyUpgradeData(srcDir, dstDir, mainDB, metricsDB, backupDir, stagingDir string) error {
	mainDB, _ = filepath.Abs(mainDB)
	metricsDB, _ = filepath.Abs(metricsDB)
	backupDir, _ = filepath.Abs(backupDir)
	stagingDir, _ = filepath.Abs(stagingDir)
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		absPath, _ := filepath.Abs(path)
		if absPath == backupDir || absPath == stagingDir {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if absPath == mainDB || absPath == metricsDB ||
			absPath == mainDB+"-wal" || absPath == mainDB+"-shm" ||
			absPath == metricsDB+"-wal" || absPath == metricsDB+"-shm" {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeInErr := in.Close()
		closeOutErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInErr != nil {
			return closeInErr
		}
		return closeOutErr
	})
}

func snapshotSQLiteConnection(db *sql.DB, destPath string) error {
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	safePath := strings.ReplaceAll(filepath.ToSlash(destPath), "'", "''")
	_, err := db.ExecContext(ctx, "VACUUM INTO '"+safePath+"'")
	return err
}

// readSystemVersion 读取配置库中的版本标记。
//
// backupOnVersionUpgrade 现在在 config.SetDb 之前执行（必须在破坏性迁移前备份），
// 因此不能走 config.GetAs；这里直接通过全局 gorm 实例读取 configs 表。
// configs 表可能尚未建表（全新安装），此时返回空版本。
func readSystemVersion() (string, error) {
	if instance == nil {
		return "", fmt.Errorf("main database is not initialized")
	}
	if !instance.Migrator().HasTable(&appconfig.ConfigItem{}) {
		return "", nil
	}
	var item appconfig.ConfigItem
	if err := instance.Where("key = ?", SystemVersionKey).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	// 版本标记以 JSON 字符串形式存储（config.Set / strconv.Quote 均带引号），
	// 比较前必须解码，否则每次启动都会误判为版本变化并重复做升级备份。
	if unquoted, uerr := strconv.Unquote(item.Value); uerr == nil {
		return unquoted, nil
	}
	return item.Value, nil
}

// writeVersionMarker 将当前 versionID 写入配置库.
// 注意：backupOnVersionUpgrade 在 config.SetDb 之前执行（必须在破坏性迁移前
// 备份），此时 config 包尚未注入数据库，因此这里直接通过全局 gorm 实例 upsert，
// 不能走 config.Set。
func writeVersionMarker() {
	if instance == nil {
		logger.Errorf("dbcore", "[upgrade-backup] cannot persist version marker: main database is not initialized")
		return
	}
	item := appconfig.ConfigItem{
		Key:   SystemVersionKey,
		Value: strconv.Quote(versionID),
	}
	if err := instance.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&item).Error; err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to persist version marker: %v", err)
	}
}

func buildSQLiteDSN(databaseFile string) string {
	if databaseFile == "" {
		databaseFile = "./data/komari.db"
	}

	params := fmt.Sprintf("_busy_timeout=%d&_txlock=immediate", mainSQLiteBusyTimeout.Milliseconds())
	separator := "?"
	if strings.Contains(databaseFile, "?") {
		separator = "&"
	}

	if strings.HasPrefix(databaseFile, "file:") {
		return databaseFile + separator + params
	}

	if databaseFile == ":memory:" {
		// 不使用 cache=shared：主库固定单连接（SetMaxOpenConns(1)），
		// 共享缓存没有意义，且其表级锁（SQLITE_LOCKED，busy_timeout 对其
		// 无效）与 metricstore/config.go 阐述的问题一致。
		return "file::memory:?" + params
	}

	return "file:" + filepath.ToSlash(databaseFile) + separator + params
}

func mainSQLiteOptions() sqlitetune.Options {
	return sqlitetune.Options{
		BusyTimeout:           mainSQLiteBusyTimeout,
		CacheSizeKB:           mainSQLiteCacheSizeKB,
		MMapSizeBytes:         0,
		TempStoreMemory:       false,
		CacheSpill:            true,
		WALAutoCheckpoint:     mainSQLiteWALAutoCheckpoint,
		JournalSizeLimitBytes: mainSQLiteJournalSizeLimit,
		Synchronous:           sqlitetune.SynchronousNormal,
	}
}

// Initialize 显式初始化数据库连接与表结构，仅执行一次。
// 与 GetDBInstance 不同，Initialize 返回错误而非直接退出进程，
// 便于启动生命周期统一处理错误、以及在测试/CLI 命令中做隔离。
func Initialize() error {
	once.Do(func() {
		initErr = doInitialize()
	})
	return initErr
}

// GetDBInstance 返回全局数据库实例。
// 为兼容既有大量调用点，这里保留“出错即退出”的语义；
// 需要错误处理的启动流程应优先调用 Initialize()。
func GetDBInstance() *gorm.DB {
	if err := Initialize(); err != nil {
		logger.Fatalf("dbcore", "Failed to initialize database: %v", err)
	}
	return instance
}

// Close 关闭底层数据库连接，供关闭流程调用。
func Close() error {
	if instance == nil {
		return nil
	}
	sqlDB, err := instance.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func doInitialize() error {
	var err error

	// 记录“打开数据库之前”komari.db 是否已存在，用于区分全新安装与旧版升级。
	// 必须在（可能的）恢复逻辑之后、gorm.Open 之前采集：恢复会解压出旧库，
	// gorm.Open 会创建空库。
	if _, statErr := os.Stat(resolveDatabaseFile()); statErr == nil {
		dbFileExistedAtStartup = true
	}

	logConfig := &gorm.Config{
		Logger:  logger.NewGormLogger(),
		NowFunc: func() time.Time { return time.Now().UTC() },
	}

	// 根据数据库类型选择不同的连接方式
	switch flags.ApplyDatabaseTypeNormalization() {
	case flags.DatabaseTypeSQLite:
		// _txlock=immediate lets writes acquire their lock before reads can
		// turn into a lock-upgrade conflict. sqlitetune applies the remaining
		// per-connection PRAGMAs whenever database/sql opens a connection.
		dsn := buildSQLiteDSN(flags.DatabaseFile)
		sqlDB, dbErr := sqlitetune.Open(dsn, mainSQLiteOptions())
		if dbErr != nil {
			return fmt.Errorf("open SQLite connection pool: %w", dbErr)
		}
		// SQLite has one writer. Keeping exactly one durable connection also
		// keeps connection-local cache and WAL limits stable for the main DB.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(0)
		sqlDB.SetConnMaxIdleTime(0)

		instance, err = gorm.Open(sqlite.New(sqlite.Config{Conn: sqlDB}), logConfig)
		if err != nil {
			_ = sqlDB.Close()
			return fmt.Errorf("failed to connect to SQLite3 database: %w", err)
		}
		if err := instance.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			logger.Errorf("dbcore", "Failed to checkpoint SQLite WAL at startup: %v", err)
		}
	default:
		return fmt.Errorf("unsupported database type: %s (supported: %s)", flags.DatabaseType, flags.SupportedDatabaseTypes())
	}
	// 版本升级备份必须在 migrations.Run 之前执行：startup migrations 会 drop 旧表、
	// 改写时间戳，若先迁移再备份，备份里已是破坏后的数据，无法用于回滚。
	// 此处配置库尚未就绪，backupOnVersionUpgrade 通过 instance 直接读取版本标记。
	backupOnVersionUpgrade()
	if err := config.SetDb(instance); err != nil {
		if sqlDB, dbErr := instance.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		instance = nil
		return fmt.Errorf("failed to initialize config store: %w", err)
	}

	if err := migrations.Run(migrations.Context{DB: instance}); err != nil {
		// 迁移失败时关闭已打开的连接池，避免初始化失败后泄漏。
		if sqlDB, dbErr := instance.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		instance = nil
		return fmt.Errorf("failed to run startup migrations: %w", err)
	}

	// 自动迁移模型
	//
	// 注意：负载/GPU/ping 历史监控数据运行期全部走 metric store（默认 SQLite
	// ./data/metrics.db，或配置的 MySQL/PostgreSQL）。旧的 records /
	// records_long_term / gpu_records / ping_records 表不再建表、不再写入。
	// 若升级时旧表仍存在，管理员可通过 admin RPC 显式导入并清理：
	// admin:getLegacyMigrationStatus 查看旧表数据概况与迁移进度，
	// admin:runLegacyMigration 执行导入（internal/migrations 的
	// MigrateLegacyMonitoring / CompleteLegacyMonitoringMigration），
	// 导入完成后旧表会被 drop 并写入完成 marker。
	// models.Record / models.PingRecord / models.GPURecord 结构体仍作为
	// metric store 的读写 DTO 和旧表导入 DTO 保留在 models 包中。
	// models.TrafficReportNotification 同样仅为旧数据库兼容保留，不再建表。

	err = instance.AutoMigrate(
		&models.User{},
		&models.Client{},
		&models.Log{},

		&models.OfflineNotification{},
		&models.PingTask{},
		&models.MessageSenderProvider{},
		&models.ThemeConfiguration{},
	)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	if err := instance.AutoMigrate(
		&models.Session{},
	); err != nil {
		// Session 表是登录依赖：建表失败时拒绝启动，而不是带着坏表照常
		// 对外服务（那会让所有登录请求在运行期反复报错）。
		if sqlDB, dbErr := instance.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		instance = nil
		return fmt.Errorf("failed to create Session table (login depends on it): %w", err)
	}
	return nil
}
