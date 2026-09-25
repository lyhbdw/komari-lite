package metricstore

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tumb1er1376/komari-monitor-lite/pkg/metric"
)

const (
	// DefaultRollupRawRetention documents the fixed in-memory exact-sample
	// window. Samples older than one minute are losslessly byte-encoded;
	// older history is served by the persisted rollup ladder.
	DefaultRollupRawRetention = 10 * time.Minute
	DefaultRollupFinestTier   = time.Minute
	defaultRollupPointLimit   = 600

	defaultRollupMinuteRetentionMinutes     = defaultRollupPointLimit
	defaultRollupFiveMinuteRetentionMinutes = 5 * defaultRollupPointLimit
	defaultRollupHourRetentionHours         = defaultRollupPointLimit
	defaultRollupDayRetentionDays           = 100 * 365
)

// MetricStoreConfig 保存 metric store 配置。
//
// 注意：metric store 现在始终启用（旧的 metric_store_enabled 开关已废弃）。
// 未显式配置时默认使用 SQLite（./data/metrics.db）。
type MetricStoreConfig struct {
	Driver       string `json:"metric_db_driver" default:"sqlite"`         // 数据库类型: sqlite, mysql, postgresql
	DSN          string `json:"metric_db_dsn" default:"./data/metrics.db"` // 数据库连接串
	TablePrefix  string `json:"metric_table_prefix" default:"metric_"`     // 表名前缀
	MaxOpenConns int    `json:"metric_max_open_conns" default:"25"`        // 最大连接数
	MaxIdleConns int    `json:"metric_max_idle_conns" default:"5"`         // 最大空闲连接数
	// RollupMinuteRetentionMinutes controls the persisted 1-minute bucket window.
	RollupMinuteRetentionMinutes int `json:"metric_rollup_minute_retention_minutes" default:"600"`
	// RollupFiveMinuteRetentionMinutes controls the persisted 5-minute bucket window.
	RollupFiveMinuteRetentionMinutes int `json:"metric_rollup_five_minute_retention_minutes" default:"3000"`
	// RollupHourRetentionHours controls the persisted 1-hour bucket window.
	RollupHourRetentionHours int `json:"metric_rollup_hour_retention_hours" default:"600"`
}

// MetricStoreConfigKeys 配置键
const (
	MetricDBDriverKey                         = "metric_db_driver"
	MetricDBDSNKey                            = "metric_db_dsn"
	MetricTablePrefixKey                      = "metric_table_prefix"
	MetricMaxOpenConnsKey                     = "metric_max_open_conns"
	MetricMaxIdleConnsKey                     = "metric_max_idle_conns"
	MetricRollupMinuteRetentionMinutesKey     = "metric_rollup_minute_retention_minutes"
	MetricRollupFiveMinuteRetentionMinutesKey = "metric_rollup_five_minute_retention_minutes"
	MetricRollupHourRetentionHoursKey         = "metric_rollup_hour_retention_hours"
)

func buildMetricConfig(cfg *MetricStoreConfig, autoMigrate bool) (metric.Config, error) {
	if cfg == nil {
		return metric.Config{}, fmt.Errorf("metric store config is nil")
	}
	configuredDriver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if configuredDriver != "" && configuredDriver != string(metric.DriverSQLite) {
		return metric.Config{}, fmt.Errorf("monitoring-only build supports SQLite metrics storage only, got %q", cfg.Driver)
	}
	if isExternalMetricDSN(cfg.DSN) {
		return metric.Config{}, fmt.Errorf("monitoring-only build rejects external metrics DSN")
	}

	tablePrefix := cfg.TablePrefix
	if tablePrefix == "" {
		tablePrefix = "metric_"
	}
	opts := []metric.Option{
		metric.WithTablePrefix(tablePrefix),
		metric.WithAutoMigrate(autoMigrate),
	}
	policy, err := rollupPolicyFromConfig(cfg)
	if err != nil {
		return metric.Config{}, err
	}
	opts = append(opts, metric.WithRollupPolicy(policy))

	dsn := cfg.DSN
	if dsn == "" || dsn == "./data/metrics.db" {
		// 注意：刻意不使用 cache=shared。SQLite 共享缓存模式使用表级锁，
		// 当一个连接持有读锁、另一个连接尝试写入时会立即返回
		// SQLITE_LOCKED（"database table is locked"），且 busy_timeout
		// 对共享缓存的表级锁无效，迁移期间与前台查询/实时写入并发时必然报错。
		// _txlock=immediate 让写事务开始即获取写锁，避免锁升级死锁。
		dsn = "file:./data/metrics.db?mode=rwc&_txlock=immediate"
	} else {
		// 用户自定义 DSN 时，剥离 cache=shared，避免上述表级锁问题。
		dsn = stripSharedCache(dsn)
	}
	// SQLite 串行化写入：固定单写连接以避免 "database is locked" 竞争，
	// 同时启用独立的 WAL 只读连接池提升前台查询并发（写仍走单主连接）。
	// 这里刻意忽略 cfg.MaxOpenConns/MaxIdleConns —— 对 SQLite 而言多写连接
	// 只会引入锁竞争而非提升吞吐。
	opts = append(opts, metric.WithMaxOpenConns(1), metric.WithMaxIdleConns(1))
	opts = append(opts, metric.WithSQLiteReadPool(2))
	return metric.SQLite(dsn, opts...), nil
}

func rollupPolicyFromConfig(cfg *MetricStoreConfig) (metric.RollupPolicy, error) {
	if cfg == nil {
		return metric.RollupPolicy{}, fmt.Errorf("metric store config is nil")
	}

	minuteRetention := cfg.RollupMinuteRetentionMinutes
	fiveMinuteRetention := cfg.RollupFiveMinuteRetentionMinutes
	hourRetention := cfg.RollupHourRetentionHours
	// Configs constructed by older callers do not have the new fields. Treat
	// their zero values as omitted so recovery and migration remain compatible.
	if minuteRetention == 0 {
		minuteRetention = defaultRollupMinuteRetentionMinutes
	}
	if fiveMinuteRetention == 0 {
		fiveMinuteRetention = defaultRollupFiveMinuteRetentionMinutes
	}
	if hourRetention == 0 {
		hourRetention = defaultRollupHourRetentionHours
	}
	if minuteRetention < 0 || fiveMinuteRetention < 0 || hourRetention < 0 {
		return metric.RollupPolicy{}, fmt.Errorf("metric rollup retention values must be positive integers")
	}

	minuteDuration, err := rollupDuration(minuteRetention, time.Minute)
	if err != nil {
		return metric.RollupPolicy{}, err
	}
	fiveMinuteDuration, err := rollupDuration(fiveMinuteRetention, time.Minute)
	if err != nil {
		return metric.RollupPolicy{}, err
	}
	hourDuration, err := rollupDuration(hourRetention, time.Hour)
	if err != nil {
		return metric.RollupPolicy{}, err
	}

	policy := rollupPolicyFromDurations(minuteDuration, fiveMinuteDuration, hourDuration)
	if err := policy.Validate(); err != nil {
		return metric.RollupPolicy{}, fmt.Errorf("invalid metric rollup retention policy: %w", err)
	}
	return policy, nil
}

func rollupPolicyFromDurations(minuteRetention, fiveMinuteRetention, hourRetention time.Duration) metric.RollupPolicy {
	return metric.RollupPolicy{
		RawRetention: DefaultRollupRawRetention,
		Tiers: []metric.RollupTier{
			{Interval: time.Minute, Retention: minuteRetention},
			{Interval: 5 * time.Minute, Retention: fiveMinuteRetention},
			{Interval: time.Hour, Retention: hourRetention},
			// Daily buckets form the terminal tier. They remain available until
			// the metric's own retention policy removes them.
			{Interval: 24 * time.Hour, Retention: time.Duration(defaultRollupDayRetentionDays) * 24 * time.Hour},
		},
		Compression: 30,
	}
}

func rollupDuration(value int, unit time.Duration) (time.Duration, error) {
	maxDurationValue := int64((time.Duration(1<<63 - 1)) / unit)
	if value <= 0 || int64(value) > maxDurationValue {
		return 0, fmt.Errorf("metric rollup retention value must be a positive duration")
	}
	return time.Duration(value) * unit, nil
}

func isExternalMetricDSN(dsn string) bool {
	raw := strings.TrimSpace(strings.ToLower(dsn))
	if raw == "" || raw == ":memory:" {
		return false
	}
	if strings.HasPrefix(raw, "file:") || strings.HasPrefix(raw, "sqlite://") || strings.HasPrefix(raw, "sqlite3://") {
		return false
	}
	if strings.HasPrefix(raw, "mysql://") || strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") {
		return true
	}
	if strings.Contains(raw, "@tcp(") || strings.Contains(raw, "@unix(") || strings.Contains(raw, "@/") {
		return true
	}
	if strings.Contains(raw, "dbname=") {
		return true
	}
	if strings.Contains(raw, "host=") && strings.Contains(raw, "user=") {
		return true
	}
	return false
}

// stripSharedCache 从 SQLite DSN 中移除 cache=shared 参数，避免共享缓存模式下的
// 表级锁（SQLITE_LOCKED "database table is locked"）。其它参数保持不变。
func stripSharedCache(dsn string) string {
	if !strings.Contains(dsn, "cache=shared") {
		return dsn
	}
	idx := strings.Index(dsn, "?")
	if idx < 0 {
		return dsn
	}
	base := dsn[:idx]
	query := dsn[idx+1:]
	parts := strings.Split(query, "&")
	kept := parts[:0]
	for _, p := range parts {
		if p == "cache=shared" {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return base
	}
	return base + "?" + strings.Join(kept, "&")
}
