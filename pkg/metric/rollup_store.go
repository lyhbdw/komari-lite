package metric

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// rollupKey identifies a materialized bucket. The tags and labels themselves
// are interned in separate dictionaries; the hashes only keep hot maps small.
type rollupKey struct {
	entityID   string
	tagsHash   string
	labelsHash string
	bucket     int64 // Unix milliseconds
}

type storedRollup struct {
	entityID   string
	bucket     int64 // Unix milliseconds
	bucketData *rollupBucket
}

// Compact seals closed minute buckets, materializes due in-memory parents, and
// enforces retention for explicit callers. The server's frequent compact task
// uses Flush and FlushCoarse only; retention runs on its own hourly schedule.
func (s *Store) Compact(ctx context.Context, now time.Time) (int, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	written, err := s.Flush(ctx, now)
	if err != nil {
		return 0, err
	}
	coarse, err := s.FlushCoarse(ctx, now)
	if err != nil {
		return written, err
	}
	written += coarse
	if _, err := s.CleanupExpired(ctx, now); err != nil {
		return written, err
	}
	return written, nil
}

// Flush seals closed in-memory minute buckets and trims the exact raw window,
// without running persisted retention or materializing coarse parents.
// It is used by the scheduled compactor so a series that stops reporting is
// still persisted even when no later report arrives to close its last minute.
func (s *Store) Flush(ctx context.Context, now time.Time) (int, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	s.retentionMu.RLock()
	defer s.retentionMu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()
	now = now.UTC()
	s.trimRawWindow(now)
	written, err := s.flushClosedHotRollups(ctx, now)
	if err != nil {
		return 0, err
	}
	return written, nil
}

func (s *Store) CompactMetric(ctx context.Context, metricName string, now time.Time) (int, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	s.retentionMu.RLock()
	defer s.retentionMu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.enforceMetricRetentionTx(ctx, metricName, now.UTC(), tx); err != nil {
		return 0, err
	}
	return 0, tx.Commit()
}

// writeTierCascadeTx persists the durable minute tier. Coarser parents are
// accumulated in memory and materialized only after their late-arrival grace.
func (s *Store) writeTierCascadeTx(ctx context.Context, metricName string, policy RollupPolicy, minute map[rollupKey]*rollupBucket, tx *sql.Tx) (int, error) {
	if len(policy.Tiers) == 0 {
		return 0, nil
	}
	return s.mergeRollupBucketsWithDictionaryTx(ctx, metricName, policy.Tiers[0].Interval, minute, newRollupDictionaryCache(), tx)
}

// replaceMinuteRollupsTx overwrites rebuilt minute buckets in place. Coarser
// ancestors are deliberately left untouched: they are accumulated in memory
// and only materialized after their late-arrival grace, where a rebuilt child
// replaces its earlier contribution. This keeps late raw upserts idempotent
// even though t-digests cannot remove observations.
func (s *Store) replaceMinuteRollupsTx(ctx context.Context, metricName string, policy RollupPolicy, replacements map[rollupKey]*rollupBucket, tx *sql.Tx) error {
	if len(replacements) == 0 || len(policy.Tiers) == 0 {
		return nil
	}
	cache := newRollupDictionaryCache()
	keys := make([]rollupKey, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sortRollupKeys(keys)
	for _, key := range keys {
		bucket := replacements[key]
		if bucket == nil || bucket.count == 0 {
			if err := s.deleteRollupBucketTx(ctx, metricName, policy.Tiers[0].Interval, key, tx); err != nil {
				return err
			}
			continue
		}
		if err := s.upsertRollupWithDictionaryTx(ctx, metricName, policy.Tiers[0].Interval, key, bucket, cache, tx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) deleteRollupBucketTx(ctx context.Context, metricName string, interval time.Duration, key rollupKey, tx *sql.Tx) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE bucket_milli = %s
		AND resolution_id IN (SELECT id FROM %s WHERE resolution_milli = %s)
		AND series_id IN (SELECT id FROM %s WHERE metric_name = %s AND entity_id = %s AND tags_hash = %s)
		AND label_id IN (SELECT id FROM %s WHERE labels_hash = %s)`,
		s.tables.rollups, s.dialect.placeholder(1),
		s.tables.resolutions, s.dialect.placeholder(2),
		s.tables.series, s.dialect.placeholder(3), s.dialect.placeholder(4), s.dialect.placeholder(5),
		s.tables.labels, s.dialect.placeholder(6))
	_, err := tx.ExecContext(ctx, query, key.bucket, interval.Milliseconds(), metricName, key.entityID, key.tagsHash, key.labelsHash)
	return err
}


// mergeRollupBatchSize bounds how many buckets share one existing-row SELECT
// and one multi-row UPSERT. Fifteen bound values per row keep a batch well
// under SQLite's default 999-variable limit, matching the coarse write path.
const mergeRollupBatchSize = 60

func (s *Store) mergeRollupBucketsWithDictionaryTx(ctx context.Context, metricName string, interval time.Duration, buckets map[rollupKey]*rollupBucket, cache *rollupDictionaryCache, tx *sql.Tx) (int, error) {
	if len(buckets) == 0 {
		return 0, nil
	}
	keys := make([]rollupKey, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sortRollupKeys(keys)
	for start := 0; start < len(keys); start += mergeRollupBatchSize {
		end := start + mergeRollupBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := s.mergeRollupBucketBatchTx(ctx, metricName, interval, keys[start:end], buckets, cache, tx); err != nil {
			return 0, err
		}
	}
	return len(keys), nil
}

// mergeRollupBucketBatchTx merges one bounded batch of buckets: a single
// SELECT loads any existing rows, then a single multi-row UPSERT writes the
// merged results, replacing the historical per-bucket SELECT+UPSERT pair.
func (s *Store) mergeRollupBucketBatchTx(ctx context.Context, metricName string, interval time.Duration, keys []rollupKey, buckets map[rollupKey]*rollupBucket, cache *rollupDictionaryCache, tx *sql.Tx) error {
	resolutionID, err := cache.resolutionID(ctx, s, tx, interval)
	if err != nil {
		return err
	}
	existing, err := s.readRollupBucketsBatchTx(ctx, metricName, interval, resolutionID, keys, tx)
	if err != nil {
		return err
	}
	rows := make([]normalizedRollupRow, 0, len(keys))
	for _, key := range keys {
		// Look up by the caller's original (possibly unnormalized) bucket
		// before normalizing the key for the dictionary and row write.
		bucket := buckets[key]
		key.bucket = normalizeBucketMillis(key.bucket)
		if stored := existing[key]; stored != nil {
			stored.mergeStored(bucket)
			bucket = stored
		}
		if bucket == nil {
			continue
		}
		seriesID, err := cache.seriesID(ctx, s, tx, metricName, key, bucket.tagsJSON)
		if err != nil {
			return err
		}
		labelID, err := cache.labelID(ctx, s, tx, key.labelsHash, bucket.labelsJSON)
		if err != nil {
			return err
		}
		rows = append(rows, normalizedRollupRow{
			seriesID: seriesID, resolutionID: resolutionID, labelID: labelID,
			bucketMilli: key.bucket, count: bucket.count, sum: bucket.sum, sumSq: bucket.sumSq,
			min: bucket.min, max: bucket.max, firstVal: bucket.firstVal, firstTSMilli: bucket.firstTS,
			lastVal: bucket.lastVal, lastTSMilli: bucket.lastTS, digest: bucket.encodedDigest(),
			createdAtMilli: timeMillis(time.Now()),
		})
	}
	return s.upsertNormalizedRollupRowsTx(ctx, rows, tx)
}

// readRollupBucketsBatchTx loads the existing stored buckets for a batch of
// keys with one query, keyed by the same rollupKey the caller merges with.
func (s *Store) readRollupBucketsBatchTx(ctx context.Context, metricName string, interval time.Duration, resolutionID int64, keys []rollupKey, tx *sql.Tx) (map[rollupKey]*rollupBucket, error) {
	wanted := make(map[rollupKey]struct{}, len(keys))
	for _, key := range keys {
		wanted[rollupKey{entityID: key.entityID, tagsHash: key.tagsHash, labelsHash: key.labelsHash, bucket: normalizeBucketMillis(key.bucket)}] = struct{}{}
	}
	args := []any{metricName, resolutionID}
	seriesPlaceholders := make([]string, 0, len(keys))
	seriesKeys := make(map[string][]rollupKey)
	for _, key := range keys {
		cacheKey := key.entityID + "\x00" + key.tagsHash
		if _, seen := seriesKeys[cacheKey]; !seen {
			args = append(args, key.entityID, key.tagsHash)
			seriesPlaceholders = append(seriesPlaceholders,
				"("+s.dialect.placeholder(len(args)-1)+", "+s.dialect.placeholder(len(args))+")")
		}
		seriesKeys[cacheKey] = append(seriesKeys[cacheKey], key)
	}
	bucketArgs := make([]any, 0, len(keys))
	bucketPlaceholders := make([]string, 0, len(keys))
	for _, key := range keys {
		bucketArgs = append(bucketArgs, normalizeBucketMillis(key.bucket))
		bucketPlaceholders = append(bucketPlaceholders, s.dialect.placeholder(len(args)+len(bucketArgs)))
	}
	sqlText := fmt.Sprintf(`SELECT s.entity_id, s.tags_hash, l.labels_hash, r.bucket_milli, r.count, r.sum, r.sum_sq, r.min_val, r.max_val, r.first_val, r.first_ts_milli, r.last_val, r.last_ts_milli, r.digest, s.tags, l.labels
		FROM %s r JOIN %s s ON s.id = r.series_id JOIN %s d ON d.id = r.resolution_id JOIN %s l ON l.id = r.label_id
		WHERE s.metric_name = %s AND d.id = %s AND (s.entity_id, s.tags_hash) IN (%s) AND r.bucket_milli IN (%s)`,
		s.tables.rollups, s.tables.series, s.tables.resolutions, s.tables.labels,
		s.dialect.placeholder(1), s.dialect.placeholder(2),
		joinSQL(seriesPlaceholders), joinSQL(bucketPlaceholders),
	)
	queryArgs := append(args, bucketArgs...)
	rows, err := tx.QueryContext(ctx, sqlText, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[rollupKey]*rollupBucket, len(keys))
	for rows.Next() {
		var entityID, tagsHash, labelsHash string
		var bucket, count, firstTS, lastTS int64
		var sum, sumSq, min, max, firstVal, lastVal float64
		var digest []byte
		var tags, labels any
		if err := rows.Scan(&entityID, &tagsHash, &labelsHash, &bucket, &count, &sum, &sumSq, &min, &max, &firstVal, &firstTS, &lastVal, &lastTS, &digest, &tags, &labels); err != nil {
			return nil, err
		}
		key := rollupKey{entityID: entityID, tagsHash: tagsHash, labelsHash: labelsHash, bucket: bucket}
		if _, ok := wanted[key]; !ok {
			continue
		}
		tagsJSON, err := rawJSONToString(tags)
		if err != nil {
			return nil, err
		}
		labelsJSON, err := rawJSONToString(labels)
		if err != nil {
			return nil, err
		}
		d, err := digestFromRollup(count, min, max, digest, s.cfg.RollupPolicy.compression())
		if err != nil {
			return nil, err
		}
		out[key] = &rollupBucket{count: count, sum: sum, sumSq: sumSq, min: min, max: max, firstVal: firstVal, firstTS: firstTS, lastVal: lastVal, lastTS: lastTS, digest: d, tagsHash: tagsHash, tagsJSON: tagsJSON, labelsHash: labelsHash, labelsJSON: labelsJSON}
	}
	return out, rows.Err()
}

// writeRollupBucketsTx is retained for package callers and tests. It uses the
// same merge-safe path as normal sealed-minute writes.
func (s *Store) writeRollupBucketsTx(ctx context.Context, metricName string, interval time.Duration, buckets map[rollupKey]*rollupBucket, tx *sql.Tx) (int, error) {
	return s.mergeRollupBucketsWithDictionaryTx(ctx, metricName, interval, buckets, newRollupDictionaryCache(), tx)
}

func (s *Store) scanRollupRows(ctx context.Context, q querier, metricName string, interval time.Duration) ([]storedRollup, error) {
	return s.scanRollupRowsBetweenWith(ctx, q, metricName, "", nil, interval, -1<<62, 1<<62, true)
}

func (s *Store) enforceMetricRetentionTx(ctx context.Context, metricName string, now time.Time, tx *sql.Tx) error {
	var retentionDays int
	err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT retention_days FROM %s WHERE name = %s", s.tables.definitions, s.dialect.placeholder(1)), metricName).Scan(&retentionDays)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if retentionDays == 0 {
		return s.deleteRollupsForMetricTx(ctx, metricName, tx)
	}
	metricRetention := time.Duration(retentionDays) * 24 * time.Hour
	policy := s.cfg.RollupPolicy.withMetricRetention(metricRetention)
	retained := make(map[time.Duration]time.Duration, len(policy.Tiers))
	for _, tier := range policy.Tiers {
		retained[tier.Interval] = tier.Retention
	}
	for _, tier := range s.cfg.RollupPolicy.Tiers {
		retention, keep := retained[tier.Interval]
		if !keep {
			if err := s.deleteRollupTierTx(ctx, metricName, tier.Interval, tx); err != nil {
				return err
			}
			continue
		}
		if err := s.deleteRollupsBeforeTx(ctx, metricName, tier.Interval, now.Add(-retention).UnixMilli(), tx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) deleteRollupTierTx(ctx context.Context, metricName string, interval time.Duration, tx *sql.Tx) error {
	sqlText := fmt.Sprintf(`DELETE FROM %s WHERE resolution_id IN (SELECT id FROM %s WHERE resolution_milli = %s) AND series_id IN (SELECT id FROM %s WHERE metric_name = %s)`,
		s.tables.rollups, s.tables.resolutions, s.dialect.placeholder(1), s.tables.series, s.dialect.placeholder(2))
	_, err := tx.ExecContext(ctx, sqlText, interval.Milliseconds(), metricName)
	return err
}

func (s *Store) deleteRollupsBeforeTx(ctx context.Context, metricName string, interval time.Duration, beforeMilli int64, tx *sql.Tx) error {
	sqlText := fmt.Sprintf(`DELETE FROM %s WHERE resolution_id IN (SELECT id FROM %s WHERE resolution_milli = %s) AND series_id IN (SELECT id FROM %s WHERE metric_name = %s) AND bucket_milli < %s`,
		s.tables.rollups, s.tables.resolutions, s.dialect.placeholder(1), s.tables.series, s.dialect.placeholder(2), s.dialect.placeholder(3))
	// Keep a bucket that straddles the cutoff because it can contain samples
	// still inside retention. At most one extra bucket per series is retained.
	beforeMilli = bucketStartMillis(beforeMilli, interval.Milliseconds())
	_, err := tx.ExecContext(ctx, sqlText, interval.Milliseconds(), metricName, beforeMilli)
	return err
}

func (s *Store) deleteRollupsForMetricTx(ctx context.Context, metricName string, tx *sql.Tx) error {
	sqlText := fmt.Sprintf("DELETE FROM %s WHERE series_id IN (SELECT id FROM %s WHERE metric_name = %s)", s.tables.rollups, s.tables.series, s.dialect.placeholder(1))
	_, err := tx.ExecContext(ctx, sqlText, metricName)
	return err
}

func sortRollupKeys(keys []rollupKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucket != keys[j].bucket {
			return keys[i].bucket < keys[j].bucket
		}
		if keys[i].entityID != keys[j].entityID {
			return keys[i].entityID < keys[j].entityID
		}
		if keys[i].tagsHash != keys[j].tagsHash {
			return keys[i].tagsHash < keys[j].tagsHash
		}
		return keys[i].labelsHash < keys[j].labelsHash
	})
}

func bucketStartMillis(ts, size int64) int64 {
	if size <= 0 {
		return ts
	}
	q := ts / size
	if ts < 0 && ts%size != 0 {
		q--
	}
	return q * size
}

func normalizeBucketMillis(bucket int64) int64 {
	if bucket > 10_000_000_000_000 || bucket < -10_000_000_000_000 {
		return bucket / int64(time.Millisecond)
	}
	return bucket
}
