package metricstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	logger "github.com/komari-monitor/komari/utils/log"

	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/metric"
)

var (
	store             *metric.Store
	storeFingerprint  string
	storeMu           sync.RWMutex
	storeInitMu       sync.Mutex
	storeOperations   = newStoreOperationGate()
	compactOperations = newStoreOperationGate()
)

var ErrCompactInProgress = errors.New("metric store compact already in progress")

// openStore 按配置打开 metric store 并创建指标定义。
func openStore(ctx context.Context, cfg *MetricStoreConfig) (*metric.Store, error) {
	return openStoreWithDefaultRetention(ctx, cfg, defaultBuiltinMetricRetentionDays)
}

func openStoreWithDefaultRetention(ctx context.Context, cfg *MetricStoreConfig, defaultRetentionDays int) (*metric.Store, error) {
	metricCfg, err := buildMetricConfig(cfg, true)
	if err != nil {
		return nil, err
	}

	s, err := metric.Open(ctx, metricCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open metric store: %w", err)
	}

	if err := createMetricDefinitionsWithDefaultRetention(ctx, s, defaultRetentionDays); err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to create metric definitions: %w", err)
	}

	return s, nil
}

// InitializeStore 初始化 metric store（启动时调用，可在失败后重试）。
func InitializeStore() error {
	storeInitMu.Lock()
	defer storeInitMu.Unlock()

	// A previous failed connection must remain retryable. The old sync.Once
	// implementation consumed the one-time call on failure and returned nil on
	// every later call while the store was still nil.
	storeMu.RLock()
	initialized := store != nil
	storeMu.RUnlock()
	if initialized {
		return nil
	}

	cfg, err := config.GetManyAs[MetricStoreConfig]()
	if err != nil {
		return fmt.Errorf("failed to load metric store config: %w", err)
	}

	// metric store 始终启用；未配置时默认 SQLite（./data/metrics.db）。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}

	storeMu.Lock()
	store = s
	storeMu.Unlock()

	logger.Infof("metricstore", "Metric store initialized successfully (driver=sqlite)")
	return nil
}

// GetStore 获取 metric store 实例（如果未启用返回 nil）
func GetStore() *metric.Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return store
}

// CloseStoreContext stops the asynchronous store migration before taking the
// store write lock, so shutdown cannot wait forever on the migration's lease.
func CloseStoreContext(ctx context.Context) error {
	if err := storeOperations.Acquire(ctx); err != nil {
		return fmt.Errorf("wait for metric store operations before close: %w", err)
	}
	defer storeOperations.Release()

	storeMu.Lock()
	defer storeMu.Unlock()

	if store != nil {
		err := store.Close()
		store = nil
		return err
	}
	return nil
}
