package jsonrpc

import (
	"context"

	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

// admin.memory.go
// 只读的进程内存预算视图。
//
// 背景：容器内存上限（256MiB）由 cgroup 强制，而 Lite 版刻意关闭了 pprof
// 路由（见 registerLiteDisabledRoutes）。当 cgroup memory.peak 触顶时，
// 操作者没有任何手段判断内存被哪一层占着。本 RPC 一次性给出全部归因：
// metric store 的进程内层级形态、SQLite 连接池占用、摄入队列积压、Go
// 运行时账目与历史查询槽占用。
//
// 边界：它只读，不触碰存储，不要求 2FA；容器 cgroup 数据在进程之外（anon
// vs file cache、内核内存、SQLite 原生分配），需要与宿主侧 memory.peak
// 配对阅读。结构体大小估算不是 RSS 实测值。
func init() {
	RegisterWithGroupAndMeta("getMemoryStats", rpc.RoleAdmin, adminGetMemoryStats, &rpc.MethodMeta{
		Name:    "admin:getMemoryStats",
		Summary: "Read-only process memory budget: metric store tiers, SQLite pools, ingestion queues, Go runtime",
		Returns: "{ store_ready: boolean, metric: {...}, pools: {...}, queues: {...}, go: {...}, history_query_slots: {...} }",
	})
}

func adminGetMemoryStats(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	snapshot := metricstore.TakeRuntimeMemorySnapshot()
	return map[string]any{
		"store_ready": metricstore.StoreReady(),
		"metric": map[string]any{
			"raw_series":              snapshot.Metric.RawSeries,
			"raw_samples":             snapshot.Metric.RawSamples,
			"raw_compressed_series":   snapshot.Metric.RawCompressedSeries,
			"raw_compressed_bytes":    snapshot.Metric.RawCompressedBytes,
			"hot_buckets":             snapshot.Metric.HotBuckets,
			"hot_digest_centroids":    snapshot.Metric.HotDigestCentroids,
			"coarse_parents":          snapshot.Metric.CoarseParents,
			"coarse_children":         snapshot.Metric.CoarseChildren,
			"coarse_digest_centroids": snapshot.Metric.CoarseDigestCentroids,
			"estimated_raw_bytes":     snapshot.Metric.EstimatedRawBytes,
			"estimated_hot_bytes":     snapshot.Metric.EstimatedHotBytes,
			"estimated_coarse_bytes":  snapshot.Metric.EstimatedCoarseBytes,
		},
		"pools": map[string]any{
			"max_open_connections":              snapshot.Pools.MaxOpenConnections,
			"open_connections":                  snapshot.Pools.OpenConnections,
			"in_use":                            snapshot.Pools.InUse,
			"idle":                              snapshot.Pools.Idle,
			"wait_count":                        snapshot.Pools.WaitCount,
			"wait_duration_ms":                  snapshot.Pools.WaitDurationMS,
			"read_pool_max_open_connections":    snapshot.Pools.ReadPoolMaxOpenConnections,
			"read_pool_open_connections":        snapshot.Pools.ReadPoolOpenConnections,
			"read_pool_in_use":                  snapshot.Pools.ReadPoolInUse,
			"read_pool_idle":                    snapshot.Pools.ReadPoolIdle,
		},
		"queues": map[string]any{
			"reports_queued":   snapshot.Queues.ReportsQueued,
			"pings_queued":     snapshot.Queues.PingsQueued,
			"reports_capacity": snapshot.Queues.ReportsCapacity,
			"pings_capacity":   snapshot.Queues.PingsCapacity,
		},
		"go": map[string]any{
			"heap_alloc_bytes":         snapshot.Go.HeapAllocBytes,
			"heap_sys_bytes":           snapshot.Go.HeapSysBytes,
			"heap_released_bytes":      snapshot.Go.HeapReleasedBytes,
			"stack_sys_bytes":          snapshot.Go.StackSysBytes,
			"sys_bytes":                snapshot.Go.SysBytes,
			"num_gc":                   snapshot.Go.NumGC,
			"next_gc_bytes":            snapshot.Go.NextGCBytes,
			"soft_memory_limit_bytes":  snapshot.Go.SoftMemoryLimitBytes,
			"gc_cpu_fraction":          snapshot.Go.GCCPUFraction,
		},
		"history_query_slots": map[string]any{
			"capacity": cap(historyQuerySlots),
			"in_use":   len(historyQuerySlots),
		},
	}, nil
}
