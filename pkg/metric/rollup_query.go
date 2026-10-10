package metric

import (
	"context"
	"fmt"
	"time"
)

// Series selects one deterministic backing tier and delegates to SeriesBatch.
func (s *Store) Series(ctx context.Context, query AggregateQuery, now time.Time) ([]AggregatePoint, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	entityIDs := []string(nil)
	if query.EntityID != "" {
		entityIDs = []string{query.EntityID}
	}
	result, err := s.SeriesBatch(ctx, BatchSeriesQuery{
		Specs: []BatchSeriesSpec{{
			MetricName:     query.MetricName,
			Aggregations:   []Aggregation{query.Aggregation},
			Interval:       query.Interval,
			PreserveSeries: query.PreserveSeries,
		}},
		EntityIDs: entityIDs,
		Start:     query.Start,
		End:       query.End,
		Tags:      query.Tags,
		Order:     query.Order,
	}, now)
	if err != nil {
		return nil, err
	}
	return pageBuckets(result.Values[query.MetricName][query.Aggregation], query.BucketLimit, query.BucketOffset), nil
}

func seriesResolutionForPolicy(start time.Time, interval time.Duration, now time.Time, policy RollupPolicy) time.Duration {
	preferred := policy.Tiers[0].Interval
	if tier := bestRollupTier(policy, interval, start.UTC(), now.UTC()); tier != nil {
		return tier.Interval
	}
	for i := len(policy.Tiers) - 1; i >= 0; i-- {
		tier := policy.Tiers[i]
		if interval >= tier.Interval && interval%tier.Interval == 0 {
			preferred = tier.Interval
			break
		}
	}
	return preferred
}

func (s *Store) CompatibleSeriesInterval(start, now time.Time, interval time.Duration) time.Duration {
	if interval <= 0 || !s.cfg.RollupPolicy.Enabled() {
		return interval
	}
	policy := s.cfg.RollupPolicy
	if interval < policy.Tiers[0].Interval {
		interval = policy.Tiers[0].Interval
	}
	backing := policy.Tiers[len(policy.Tiers)-1]
	for _, tier := range policy.Tiers {
		if !now.UTC().Add(-tier.Retention).After(start.UTC()) {
			backing = tier
			break
		}
	}
	if interval <= backing.Interval {
		return backing.Interval
	}
	if remainder := interval % backing.Interval; remainder != 0 {
		interval += backing.Interval - remainder
	}
	return interval
}

func bestRollupTier(policy RollupPolicy, interval time.Duration, start, now time.Time) *RollupTier {
	var best *RollupTier
	for i := range policy.Tiers {
		tier := &policy.Tiers[i]
		if interval >= tier.Interval && interval%tier.Interval == 0 && !now.Add(-tier.Retention).After(start) {
			best = tier
		}
	}
	return best
}

func rawJSONToString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "{}", nil
	case string:
		if v == "" {
			return "{}", nil
		}
		return v, nil
	case []byte:
		if len(v) == 0 {
			return "{}", nil
		}
		return string(v), nil
	default:
		return "", fmt.Errorf("unsupported JSON column type %T", value)
	}
}

func rollupTagsFromJSON(raw string) (map[string]string, error) {
	values, err := decodeMapString(raw)
	if err != nil {
		return nil, err
	}
	return cloneStringMap(values), nil
}
