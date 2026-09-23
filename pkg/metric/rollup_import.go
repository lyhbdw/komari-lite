package metric

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// ReplaceRollupPoints idempotently imports pre-aggregated representative
// values into the configured tiers through interval. Keeping the same
// representative in finer retained tiers prevents a newly migrated series
// from appearing blank while queries still prefer those tiers. Imported
// values never enter the exact ten-minute raw window.
func (s *Store) ReplaceRollupPoints(ctx context.Context, interval time.Duration, points []Point) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !s.hasRollupTier(interval) {
		return fmt.Errorf("%w: rollup interval %s is not configured", ErrInvalidArgument, interval)
	}
	if len(points) == 0 {
		return nil
	}
	s.retentionMu.RLock()
	defer s.retentionMu.RUnlock()
	points, err := s.filterDisabledMetricPoints(ctx, points)
	if err != nil || len(points) == 0 {
		return err
	}
	prepared, err := prepareMetricPoints(points)
	if err != nil {
		return err
	}

	tiers := make([]time.Duration, 0, len(s.cfg.RollupPolicy.Tiers))
	for _, tier := range s.cfg.RollupPolicy.Tiers {
		if tier.Interval <= interval {
			tiers = append(tiers, tier.Interval)
		}
	}
	type importGroup struct {
		metricName string
		interval   time.Duration
		buckets    map[rollupKey]*rollupBucket
	}
	groups := make(map[string]*importGroup)
	for _, point := range prepared {
		for _, tier := range tiers {
			key := rollupKey{
				entityID: point.entityID, tagsHash: point.tagsHash, labelsHash: point.labelsHash,
				bucket: bucketStartMillis(point.timestamp, tier.Milliseconds()),
			}
			groupKey := point.metricName + "\x00" + tier.String()
			group := groups[groupKey]
			if group == nil {
				group = &importGroup{metricName: point.metricName, interval: tier, buckets: make(map[rollupKey]*rollupBucket)}
				groups[groupKey] = group
			}
			if _, exists := group.buckets[key]; exists {
				return fmt.Errorf("%w: duplicate pre-aggregated point for metric %q at %s", ErrInvalidArgument, point.metricName, fromMillis(key.bucket))
			}
			bucket := newRollupBucket(s.cfg.RollupPolicy.compression())
			bucket.tagsHash, bucket.tagsJSON = point.tagsHash, point.tagsJSON
			bucket.labelsHash, bucket.labelsJSON = point.labelsHash, point.labelsJSON
			bucket.addPoint(point.value, point.timestamp)
			group.buckets[key] = bucket
		}
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	cache := newRollupDictionaryCache()
	for _, groupKey := range groupKeys {
		group := groups[groupKey]
		keys := make([]rollupKey, 0, len(group.buckets))
		for key := range group.buckets {
			keys = append(keys, key)
		}
		sortRollupKeys(keys)
		for _, key := range keys {
			if err := s.upsertRollupWithDictionaryTx(ctx, group.metricName, group.interval, key, group.buckets[key], cache, tx); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) hasRollupTier(interval time.Duration) bool {
	for _, tier := range s.cfg.RollupPolicy.Tiers {
		if tier.Interval == interval {
			return true
		}
	}
	return false
}
