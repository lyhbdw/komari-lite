package metricstore

import (
	"runtime"
	"runtime/debug"

	"github.com/lyhbdw/komari-lite/pkg/metric"
)

// RuntimeMemorySnapshot is one read-only view of everything that determines
// whether the process stays inside its container memory limit: the metric
// store's in-memory tiers, the SQLite pool occupancy, the ingestion queue
// depth and the Go runtime's own accounting.
//
// It deliberately does not include container cgroup figures. Those live
// outside the process (anon vs file cache, kernel memory, native SQLite
// allocations) and cannot be read from inside a portable Go binary; the
// snapshot is the per-tier attribution an operator pairs with cgroup
// memory.peak measured from the host.
//
// RuntimeMemorySnapshot 是决定进程能否留在容器内存限额内的全部因素的只读
// 视图：metric store 的内存层级、SQLite 连接池占用、摄入队列深度与 Go 运行时
// 自身的账目。
//
// 它刻意不含容器 cgroup 数据。那些在进程之外（anon 与 file cache、内核内存、
// SQLite 原生分配），可移植的 Go 二进制无法读取；本快照是操作者与宿主侧
// cgroup memory.peak 配对使用的逐层归因。
type RuntimeMemorySnapshot struct {
	// Metric is the store's process-local tier shape.
	//
	// Metric 是 Store 进程内各层级的形态。
	Metric metric.MemoryStats
	// Pools is the SQLite write/read pool occupancy.
	//
	// Pools 是 SQLite 写/读连接池占用。
	Pools metric.DBPoolStats
	// Queues is the outstanding ingestion backlog.
	//
	// Queues 是待摄入的积压量。
	Queues queueStats
	// Go is the Go runtime's own memory accounting.
	//
	// Go 是 Go 运行时自身的内存账目。
	Go goMemStats
}

// queueStats reports the batcher backlog. Budget is only released after a
// successful write, so a non-zero depth that never drains is the backlog
// signal; a depth pinned at the capacity is the rejection signal.
//
// queueStats 报告批处理积压。预算只在写入成功后释放，因此持续不为零的深度
// 是积压信号，长期钉在上限则是拒绝写入的信号。
type queueStats struct {
	// ReportsQueued counts queued plus pending plus in-flight reports.
	//
	// ReportsQueued 计入排队、待写与在途的报告数。
	ReportsQueued int
	// PingsQueued counts queued plus pending plus in-flight ping records.
	//
	// PingsQueued 计入排队、待写与在途的 ping 记录数。
	PingsQueued int
	// ReportsCapacity / PingsCapacity are the fixed queue capacities; depth
	// at capacity means new reports are rejected.
	//
	// ReportsCapacity / PingsCapacity 是固定队列容量；深度等于容量意味着新报告
	// 会被拒绝。
	ReportsCapacity int
	PingsCapacity   int
}

// goMemStats is the Go runtime memory accounting. These are runtime-managed
// numbers: they exclude native SQLite allocations and page cache, so they
// must be read together with cgroup totals.
//
// goMemStats 是 Go 运行时内存账目。这些是运行时管理的数字：不含 SQLite 原生
// 分配与页缓存，因此必须与 cgroup 总量一起读。
type goMemStats struct {
	// HeapAllocBytes is the live heap size.
	//
	// HeapAllocBytes 是存活堆大小。
	HeapAllocBytes uint64
	// HeapSysBytes is the heap obtained from the OS.
	//
	// HeapSysBytes 是从操作系统取得的堆大小。
	HeapSysBytes uint64
	// HeapReleasedBytes is heap returned to the OS.
	//
	// HeapReleasedBytes 是已归还给操作系统的堆。
	HeapReleasedBytes uint64
	// StackSysBytes is the goroutine stack memory.
	//
	// StackSysBytes 是 goroutine 栈内存。
	StackSysBytes uint64
	// SysBytes is total memory obtained from the OS.
	//
	// SysBytes 是从操作系统取得的内存总量。
	SysBytes uint64
	// NumGC is the completed GC cycle count; a fast-rising counter under a
	// flat heap signals allocation churn rather than a leak.
	//
	// NumGC 是已完成 GC 周期数；堆平稳而该值快速增长说明是分配抖动而非泄漏。
	NumGC uint32
	// NextGCBytes is the target heap size that triggers the next GC.
	//
	// NextGCBytes 是触发下一次 GC 的目标堆大小。
	NextGCBytes uint64
	// SoftMemoryLimitBytes is GOMEMLIMIT as seen by this process (0 when no
	// limit is configured).
	//
	// SoftMemoryLimitBytes 是本进程视角的 GOMEMLIMIT（未配置时为 0）。
	SoftMemoryLimitBytes int64
	// GCCPUFraction is the GC's share of program time since start.
	//
	// GCCPUFraction 是自启动以来 GC 占用的程序时间比例。
	GCCPUFraction float64
}

// TakeRuntimeMemorySnapshot collects the snapshot. It never blocks on the
// database or on the batcher: pool stats are atomic counters and queue depth
// reads the outstanding budget under the worker mutex.
//
// TakeRuntimeMemorySnapshot 采集快照。它不会阻塞在数据库或批处理上：连接池
// 统计是原子计数器，队列深度在 worker 互斥锁下读取未释放预算。
func TakeRuntimeMemorySnapshot() RuntimeMemorySnapshot {
	snapshot := RuntimeMemorySnapshot{}

	storeMu.RLock()
	current := store
	storeMu.RUnlock()
	if current != nil {
		snapshot.Metric = current.MemoryStats()
		snapshot.Pools = current.DBPoolStats()
	}

	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker != nil {
		worker.mu.Lock()
		snapshot.Queues = queueStats{
			ReportsQueued:   worker.reportsOutstanding,
			PingsQueued:     worker.pingsOutstanding,
			ReportsCapacity: cap(worker.queue),
			PingsCapacity:   cap(worker.pingQueue),
		}
		worker.mu.Unlock()
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snapshot.Go = goMemStats{
		HeapAllocBytes:       mem.HeapAlloc,
		HeapSysBytes:         mem.HeapSys,
		HeapReleasedBytes:    mem.HeapReleased,
		StackSysBytes:        mem.StackSys,
		SysBytes:             mem.Sys,
		NumGC:                mem.NumGC,
		NextGCBytes:          mem.NextGC,
		SoftMemoryLimitBytes: debug.SetMemoryLimit(-1),
		GCCPUFraction:        mem.GCCPUFraction,
	}
	return snapshot
}
