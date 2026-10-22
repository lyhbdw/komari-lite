package metric

import "time"

// MemoryStats is a read-only snapshot of everything the Store keeps on the
// process heap between write batches. It exists so an operator can answer
// "which in-memory tier is actually holding memory" without attaching a
// profiler: every process-local structure is counted, and the digest
// centroid count is the only place where a bounded sketch can still grow
// with cardinality.
//
// MemoryStats 是 Store 在两次写入批次之间驻留在进程堆上的全部内容的只读
// 快照。它的用途是在不挂 profiler 的情况下回答"内存到底被哪一层占着"：
// 每个进程内结构都有计数，其中 digest 质心数是唯一会随基数增长的有界
// sketch。
type MemoryStats struct {
	// RawSeries is the number of logical series in the exact-sample window.
	//
	// RawSeries 是精确样本窗口内的逻辑序列数。
	RawSeries int
	// RawSamples is the total number of retained exact samples (direct plus
	// byte-encoded).
	//
	// RawSamples 是保留的精确样本总数（可直接检索部分加字节编码部分）。
	RawSamples int
	// RawCompressedSeries is how many series keep byte-encoded samples.
	//
	// RawCompressedSeries 是多少个序列存在字节编码样本。
	RawCompressedSeries int
	// RawCompressedBytes is the exact length of every encoded byte buffer.
	//
	// RawCompressedBytes 是全部编码缓冲区的真实字节长度。
	RawCompressedBytes int
	// HotBuckets is the number of active minute buckets.
	//
	// HotBuckets 是活跃的分钟桶数量。
	HotBuckets int
	// HotDigestCentroids is the sum of centroid counts across hot buckets.
	//
	// HotDigestCentroids 是全部热桶质心数之和。
	HotDigestCentroids int
	// CoarseParents is the number of unsealed coarse parents.
	//
	// CoarseParents 是未封存的粗层父桶数量。
	CoarseParents int
	// CoarseChildren is the total number of minute children they hold.
	//
	// CoarseChildren 是它们持有的分钟子桶总数。
	CoarseChildren int
	// CoarseDigestCentroids is the sum of centroid counts across every
	// retained child digest.
	//
	// CoarseDigestCentroids 是全部子桶 digest 质心数之和。
	CoarseDigestCentroids int
	// EstimatedRawBytes / EstimatedHotBytes / EstimatedCoarseBytes are
	// deterministic struct-size estimates, not runtime measurements. They
	// are intended for tier comparison and change detection, not for
	// accounting against RSS: Go runtime overhead, GC headroom, native
	// SQLite allocations and page cache are all outside this accounting.
	//
	// EstimatedRawBytes / EstimatedHotBytes / EstimatedCoarseBytes 是基于
	// 结构体大小的确定性估算，不是运行时实测值。它们用于层级之间的对比和
	// 变化观测，不能与 RSS 对账：Go 运行时开销、GC 余量、SQLite 原生分配
	// 与页缓存都不在此账目内。
	EstimatedRawBytes    int64
	EstimatedHotBytes    int64
	EstimatedCoarseBytes int64
}

const (
	// estimatedRawSampleBytes is one rawSample (int64 + float64 + uint32
	// with padding).
	estimatedRawSampleBytes = 24
	// estimatedRawSeriesOverhead covers the map entry, tags string and the
	// label interning slices for one series.
	estimatedRawSeriesOverhead = 200
	// estimatedHotBucketBytes is the fixed part of one rollupBucket:
	// nine numeric fields plus four string headers and the digest pointer.
	estimatedHotBucketBytes = 144
	// estimatedCoarseParentBytes is the parent map entry and its children
	// map header.
	estimatedCoarseParentBytes = 96
	// estimatedCentroidBytes is one centroid (two float64s).
	estimatedCentroidBytes = 16
	// estimatedDigestBytes is the fixed part of one TDigest struct.
	estimatedDigestBytes = 64
	// estimatedCompressedSampleBytes is the byte-encoded form of one
	// sample: varint delta + 8 value bytes + varint label id.
	estimatedCompressedSampleBytes = 12
)

// MemoryStats returns a consistent-enough snapshot of the store's
// process-local structures. Each lock is taken and released independently so
// the snapshot never holds two of them at once and cannot invert the lock
// order used by the write path.
//
// MemoryStats 返回 Store 进程内结构的快照。每把锁都独立获取并立即释放，
// 因此快照绝不会同时持有两把锁，不会与写入路径的加锁顺序冲突。
func (s *Store) MemoryStats() MemoryStats {
	var stats MemoryStats

	s.rawMu.RLock()
	for _, series := range s.raw {
		if series == nil {
			continue
		}
		stats.RawSeries++
		stats.RawSamples += len(series.samples) + series.compressed.count
		if series.compressed.count > 0 {
			stats.RawCompressedSeries++
			stats.RawCompressedBytes += len(series.compressed.data)
		}
		stats.EstimatedRawBytes += estimatedRawSeriesOverhead
		stats.EstimatedRawBytes += int64(len(series.samples)) * estimatedRawSampleBytes
		stats.EstimatedRawBytes += int64(series.compressed.count) * estimatedCompressedSampleBytes
	}
	s.rawMu.RUnlock()

	s.hotMu.RLock()
	for _, bucket := range s.hot {
		if bucket == nil {
			continue
		}
		stats.HotBuckets++
		if bucket.digest != nil {
			stats.HotDigestCentroids += len(bucket.digest.centroids)
		}
		stats.EstimatedHotBytes += estimatedHotBucketBytes
		if bucket.digest != nil {
			stats.EstimatedHotBytes += estimatedDigestBytes +
				int64(len(bucket.digest.centroids))*estimatedCentroidBytes
		}
	}
	s.hotMu.RUnlock()

	s.coarseMu.RLock()
	for _, parent := range s.coarse {
		if parent == nil {
			continue
		}
		stats.CoarseParents++
		stats.CoarseChildren += len(parent.children)
		stats.EstimatedCoarseBytes += estimatedCoarseParentBytes
		for _, child := range parent.children {
			if child == nil {
				continue
			}
			stats.EstimatedCoarseBytes += estimatedHotBucketBytes
			if child.digest != nil {
				stats.CoarseDigestCentroids += len(child.digest.centroids)
				stats.EstimatedCoarseBytes += estimatedDigestBytes +
					int64(len(child.digest.centroids))*estimatedCentroidBytes
			}
		}
	}
	s.coarseMu.RUnlock()

	return stats
}

// DBPoolStats is the database/sql pool snapshot for both metric pools.
//
// DBPoolStats 是两个指标连接池的 database/sql 快照。
type DBPoolStats struct {
	// MaxOpenConnections is the configured write-pool ceiling. The metric
	// store deliberately fixes this at 1 for SQLite: more writers only add
	// lock contention.
	//
	// MaxOpenConnections 是配置的写连接池上限。metric store 对 SQLite 刻意固定
	// 为 1：更多写连接只会带来锁竞争。
	MaxOpenConnections int
	// OpenConnections / InUse / Idle are the current pool occupancy.
	//
	// OpenConnections / InUse / Idle 是当前连接池占用。
	OpenConnections int
	InUse           int
	Idle            int
	// WaitCount and WaitDuration report connection starvation, which is the
	// signal that queries are queuing behind the single writer.
	//
	// WaitCount 与 WaitDuration 反映连接饥饿，这是查询在单写连接后排队的信号。
	WaitCount     int64
	WaitDurationMS int64
	// ReadPoolMaxOpenConnections is the dedicated read pool ceiling (0 when
	// no read pool is configured).
	//
	// ReadPoolMaxOpenConnections 是专用读连接池上限（未配置读池时为 0）。
	ReadPoolMaxOpenConnections int
	ReadPoolOpenConnections   int
	ReadPoolInUse             int
	ReadPoolIdle              int
}

// DBPoolStats returns the live pool snapshot. It never blocks on the
// database: database/sql.Stats reads atomic counters.
//
// DBPoolStats 返回实时连接池快照。它不会阻塞在数据库上：database/sql.Stats
// 只读取原子计数器。
func (s *Store) DBPoolStats() DBPoolStats {
	write := s.db.Stats()
	stats := DBPoolStats{
		MaxOpenConnections: write.MaxOpenConnections,
		OpenConnections:    write.OpenConnections,
		InUse:              write.InUse,
		Idle:               write.Idle,
		WaitCount:          write.WaitCount,
		WaitDurationMS:     write.WaitDuration.Milliseconds(),
	}
	if s.readDB != nil {
		read := s.readDB.Stats()
		stats.ReadPoolMaxOpenConnections = read.MaxOpenConnections
		stats.ReadPoolOpenConnections = read.OpenConnections
		stats.ReadPoolInUse = read.InUse
		stats.ReadPoolIdle = read.Idle
	}
	return stats
}

// DigestRetentionHint reports the age of the oldest exact sample the store
// still holds. It is zero when the raw window is empty and is used to
// confirm that the in-memory window is actually expiring.
//
// DigestRetentionHint 报告 Store 仍持有的最老精确样本的年龄。原始窗口为空时
// 返回 0，用于确认内存窗口确实在过期。
func (s *Store) RawWindowAge(now time.Time) time.Duration {
	var oldest int64
	s.rawMu.RLock()
	for _, series := range s.raw {
		if series == nil {
			continue
		}
		for _, sample := range series.samples {
			if oldest == 0 || sample.timestamp < oldest {
				oldest = sample.timestamp
			}
		}
		if series.compressed.count > 0 {
			first := series.compressed.firstStamp()
			if oldest == 0 || first < oldest {
				oldest = first
			}
		}
	}
	s.rawMu.RUnlock()
	if oldest == 0 {
		return 0
	}
	age := now.UTC().UnixMilli() - oldest
	if age < 0 {
		return 0
	}
	return time.Duration(age) * time.Millisecond
}
