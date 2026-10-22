package metric

import (
	"context"
	"time"
)

// RetentionTierInfo uses seconds so clients need not decode Go nanoseconds.
type RetentionTierInfo struct {
	IntervalSeconds  int64  `json:"interval_seconds"`
	RetentionSeconds int64  `json:"retention_seconds"`
	Source           string `json:"source"`
}
type MetricRetentionInfo struct {
	MetricName              string              `json:"metric_name"`
	DefinitionRetentionDays int                 `json:"definition_retention_days"`
	Enabled                 bool                `json:"enabled"`
	ConfiguredTiers         []RetentionTierInfo `json:"configured_tiers"`
	EffectiveTiers          []RetentionTierInfo `json:"effective_tiers"`
}
type RetentionInfo struct {
	RawRetentionSeconds int64                 `json:"raw_retention_seconds"`
	RawSource           string                `json:"raw_source"`
	Metrics             []MetricRetentionInfo `json:"metrics"`
}

func retentionTiers(tiers []RollupTier, source string) []RetentionTierInfo {
	out := make([]RetentionTierInfo, 0, len(tiers))
	for _, tier := range tiers {
		out = append(out, RetentionTierInfo{int64(tier.Interval / time.Second), int64(tier.Retention / time.Second), source})
	}
	return out
}

// RetentionInfo inspects the active store policy and definitions without
// flushing, compacting, changing configuration, or applying retention.
func (s *Store) RetentionInfo(ctx context.Context) (RetentionInfo, error) {
	definitions, err := s.ListMetrics(ctx)
	if err != nil {
		return RetentionInfo{}, err
	}
	out := RetentionInfo{RawRetentionSeconds: int64(rawMemoryRetention / time.Second), RawSource: "fixed_memory_window", Metrics: make([]MetricRetentionInfo, 0, len(definitions))}
	policy := s.cfg.RollupPolicy
	for _, def := range definitions {
		m := MetricRetentionInfo{MetricName: def.Name, DefinitionRetentionDays: def.RetentionDays, Enabled: def.RetentionDays > 0, ConfiguredTiers: retentionTiers(policy.Tiers, "store_policy"), EffectiveTiers: []RetentionTierInfo{}}
		if m.Enabled {
			effective := policy.withMetricRetention(time.Duration(def.RetentionDays) * 24 * time.Hour)
			m.EffectiveTiers = retentionTiers(effective.Tiers, "store_policy")
			for i, tier := range effective.Tiers {
				for j, configured := range policy.Tiers {
					if tier.Interval == configured.Interval && (j == len(policy.Tiers)-1 || tier.Retention != configured.Retention) {
						m.EffectiveTiers[i].Source = "metric_definition"
					}
				}
			}
		}
		out.Metrics = append(out.Metrics, m)
	}
	return out, nil
}
