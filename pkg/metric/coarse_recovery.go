package metric

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	coarseRecoveryTimeout  = 5 * time.Second
	coarseRecoveryRowLimit = 50000
)

// Recovery is best effort and must never prevent the store becoming ready.
// Discard partial state on failure: a partial parent must not overwrite a
// complete persisted summary when it eventually seals.
func (s *Store) recoverCoarseRollupsBestEffort(ctx context.Context) {
	recoveryCtx, cancel := context.WithTimeout(ctx, coarseRecoveryTimeout)
	defer cancel()
	if err := s.recoverCoarseRollups(recoveryCtx, time.Now().UTC()); err != nil {
		s.coarse = make(map[coarseRollupKey]*coarseRollup)
		s.hotBaseline = nil
		log.Printf("metric: coarse recovery skipped: %v", err)
	}
}

// recoverCoarseRollups reconstructs process-local parents from committed child
// summaries, once at startup. It neither persists raw samples nor rewrites
// sealed parents. Missing parents stay in the usual accumulator until due.
func (s *Store) recoverCoarseRollups(ctx context.Context, now time.Time) error {
	if len(s.cfg.RollupPolicy.Tiers) < 2 {
		return nil
	}
	// Open without AutoMigrate also supports a caller creating the schema later.
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)", s.tables.rollups).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	defs, err := s.ListMetrics(ctx)
	if err != nil {
		return err
	}
	type recoveryGroup struct {
		policy RollupPolicy
		names  []string
	}
	groups := make(map[string]*recoveryGroup)
	var ordered []*recoveryGroup
	for _, def := range defs {
		if def.RetentionDays <= 0 {
			continue
		}
		policy := s.cfg.RollupPolicy.withMetricRetention(time.Duration(def.RetentionDays) * 24 * time.Hour)
		key := fmt.Sprint(policy.Tiers)
		group := groups[key]
		if group == nil {
			group = &recoveryGroup{policy: policy}
			groups[key] = group
			ordered = append(ordered, group)
		}
		group.names = append(group.names, def.Name)
	}
	for _, group := range ordered {
		for i := 1; i < len(group.policy.Tiers); i++ {
			if err := s.recoverCoarseTierGroup(ctx, group.names, group.policy, i, now); err != nil {
				return fmt.Errorf("recover metrics %q tier %s: %w", group.names, group.policy.Tiers[i].Interval, err)
			}
		}
	}
	return nil
}

func (s *Store) recoverCoarseTier(ctx context.Context, name string, policy RollupPolicy, index int, now time.Time) error {
	return s.recoverCoarseTierGroup(ctx, []string{name}, policy, index, now)
}

func (s *Store) recoverCoarseTierGroup(ctx context.Context, names []string, policy RollupPolicy, index int, now time.Time) error {
	child, parent := policy.Tiers[index-1], policy.Tiers[index]
	width := parent.Interval.Milliseconds()
	// Only parents inside the sealing window can still be process-local.
	// Historical gaps are outside the recovery contract. Bound both ends so
	// malformed future timestamps cannot expand startup work indefinitely.
	cutoff := bucketStartMillis(now.Add(-parent.Interval-coarseRollupGrace).UnixMilli(), width) + width
	retentionCutoff := bucketStartMillis(now.Add(-parent.Retention).UnixMilli(), width)
	if retentionCutoff > cutoff {
		cutoff = retentionCutoff
	}
	type persistedKey struct{ seriesID, labelID, bucket int64 }
	sealed := make(map[persistedKey]struct{})
	parentRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT r.series_id,r.label_id,r.bucket_milli FROM %s r INDEXED BY %s JOIN %s d ON d.id=r.resolution_id WHERE d.resolution_milli=? AND r.bucket_milli>=? AND r.bucket_milli<=? LIMIT ?`, s.tables.rollups, s.cfg.TablePrefix+"rollups_resolution_bucket_idx", s.tables.resolutions), width, cutoff, now.UnixMilli(), coarseRecoveryRowLimit+1)
	if err != nil {
		return err
	}
	for parentRows.Next() {
		var key persistedKey
		if err := parentRows.Scan(&key.seriesID, &key.labelID, &key.bucket); err != nil {
			parentRows.Close()
			return err
		}
		sealed[key] = struct{}{}
		if len(sealed) > coarseRecoveryRowLimit {
			parentRows.Close()
			return fmt.Errorf("parent row limit %d exceeded", coarseRecoveryRowLimit)
		}
	}
	if err := parentRows.Err(); err != nil {
		parentRows.Close()
		return err
	}
	parentRows.Close()
	args := []any{}
	placeholders := appendPlaceholders(s.dialect, &args, names)
	query := fmt.Sprintf(`SELECT r.series_id,r.label_id,s.metric_name, s.entity_id, s.tags_hash, l.labels_hash, r.bucket_milli,
		r.count, r.sum, r.sum_sq, r.min_val, r.max_val, r.first_val, r.first_ts_milli,
		r.last_val, r.last_ts_milli, r.digest, s.tags, l.labels
		FROM %s r INDEXED BY %s JOIN %s s ON s.id = r.series_id
		JOIN %s d ON d.id = r.resolution_id JOIN %s l ON l.id = r.label_id
		WHERE s.metric_name IN (%s) AND d.resolution_milli = ? AND r.bucket_milli >= ? AND r.bucket_milli <= ?
		ORDER BY r.bucket_milli LIMIT ?`,
		s.tables.rollups, s.cfg.TablePrefix+"rollups_resolution_bucket_idx", s.tables.series, s.tables.resolutions, s.tables.labels, strings.Join(placeholders, ","))
	args = append(args, child.Interval.Milliseconds(), cutoff, now.UnixMilli(), coarseRecoveryRowLimit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	higher := make([]time.Duration, 0, len(policy.Tiers)-index-1)
	for _, tier := range policy.Tiers[index+1:] {
		higher = append(higher, tier.Interval)
	}
	scanned, restored := 0, 0
	parents := make(map[coarseRollupKey]struct{})
	defer func() {
		log.Printf("metric: coarse recovery metric=%q tier=%s scanned_children=%d restored_children=%d restored_parents=%d cutoff=%s", names, parent.Interval, scanned, restored, len(parents), time.UnixMilli(cutoff).UTC().Format(time.RFC3339))
	}()
	for rows.Next() {
		scanned++
		if scanned > coarseRecoveryRowLimit {
			return fmt.Errorf("child row limit %d exceeded", coarseRecoveryRowLimit)
		}
		var name string
		var source persistedKey
		var key rollupKey
		var bucket rollupBucket
		var digest []byte
		var tags, labels any
		if err := rows.Scan(&source.seriesID, &source.labelID, &name, &key.entityID, &key.tagsHash, &key.labelsHash, &key.bucket,
			&bucket.count, &bucket.sum, &bucket.sumSq, &bucket.min, &bucket.max,
			&bucket.firstVal, &bucket.firstTS, &bucket.lastVal, &bucket.lastTS,
			&digest, &tags, &labels); err != nil {
			return err
		}
		source.bucket = bucketStartMillis(key.bucket, width)
		if _, exists := sealed[source]; exists {
			continue
		}
		restored++
		parentKey := key
		parentKey.bucket = bucketStartMillis(key.bucket, width)
		parents[coarseRollupKey{metricName: name, interval: parent.Interval, rollupKey: parentKey}] = struct{}{}
		bucket.tagsHash, bucket.labelsHash = key.tagsHash, key.labelsHash
		if bucket.tagsJSON, err = rawJSONToString(tags); err != nil {
			return err
		}
		if bucket.labelsJSON, err = rawJSONToString(labels); err != nil {
			return err
		}
		if bucket.digest, err = digestFromRollup(bucket.count, bucket.min, bucket.max, digest, policy.compression()); err != nil {
			return err
		}
		if child.Interval == time.Minute && key.bucket >= rawMemoryCutoff(now) {
			if s.hotBaseline == nil {
				s.hotBaseline = make(map[hotRollupKey]*rollupBucket)
			}
			s.hotBaseline[hotRollupKey{metricName: name, entityID: key.entityID, tagsHash: key.tagsHash, labelsHash: key.labelsHash, bucket: key.bucket}] = bucket.clone(true)
		}
		s.addCoarseChildren(name, parent.Interval, higher, map[rollupKey]*rollupBucket{key: &bucket})
	}
	return rows.Err()
}
