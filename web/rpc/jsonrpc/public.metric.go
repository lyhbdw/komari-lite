package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/metricstore"
	"github.com/lyhbdw/komari-lite/pkg/metric"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

const (
	defaultMetricQueryPoints     = 500
	maxPublicMetricQueryPoints   = 5000
	maxPublicMetricQueryHours    = 24 * 365
	maxPublicMetricQueryKeys     = 32
	maxPublicMetricQueryEntities = 128
)

func init() {
	regPublic("listMetricDefinitions", publicListMetricDefinitions, "List public metric definitions")
	regPublic("queryMetrics", publicQueryMetrics, "Query metric points")
	regPublic("getPingMetricStats", publicGetPingMetricStats, "Get ping metric statistics")
}

type publicMetricQueryParams struct {
	MetricKey  string   `json:"metric_key"`
	MetricKeys []string `json:"metric_keys"`
	Metrics    []string `json:"metrics"`

	EntityID  string   `json:"entity_id"`
	EntityIDs []string `json:"entity_ids"`

	Start     *time.Time `json:"start"`
	StartTime *time.Time `json:"start_time"`
	End       *time.Time `json:"end"`
	EndTime   *time.Time `json:"end_time"`
	Hours     float64    `json:"hours"`

	Tags map[string]string `json:"tags"`

	FillEmpty *bool `json:"fill_empty"`

	MaxPoints         int            `json:"max_points"`
	MaxPointsByMetric map[string]int `json:"max_points_by_metric"`
	PointsByMetric    map[string]int `json:"points_by_metric"`

	Aggregation         string            `json:"aggregation"`
	Algorithm           string            `json:"algorithm"`
	AggregationByMetric map[string]string `json:"aggregation_by_metric"`
	AlgorithmByMetric   map[string]string `json:"algorithm_by_metric"`
}

type metricDefinitionResponse struct {
	Name          string            `json:"name"`
	Description   any               `json:"description,omitempty"`
	Type          string            `json:"type"`
	Unit          string            `json:"unit,omitempty"`
	RetentionDays int               `json:"retention_days"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func metricDescriptionValue(raw string) any {
	desc := strings.TrimSpace(raw)
	if desc == "" {
		return ""
	}
	var dict map[string]string
	if err := json.Unmarshal([]byte(desc), &dict); err == nil && len(dict) > 0 {
		return dict
	}
	return raw
}

type publicMetricPoint struct {
	entityID string
	Time     time.Time         `json:"time"`
	Value    *float64          `json:"value"`
	Count    int               `json:"count,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type publicMetricSeries struct {
	MetricKey           string              `json:"metric_key"`
	EntityID            string              `json:"entity_id"`
	Type                string              `json:"type,omitempty"`
	Unit                string              `json:"unit,omitempty"`
	RetentionDays       int                 `json:"retention_days,omitempty"`
	Tags                map[string]string   `json:"tags,omitempty"`
	Downsampled         bool                `json:"downsampled"`
	DownsampleAlgorithm string              `json:"downsample_algorithm,omitempty"`
	FillEmpty           bool                `json:"fill_empty,omitempty"`
	MaxPoints           int                 `json:"max_points,omitempty"`
	IntervalSeconds     float64             `json:"interval_seconds,omitempty"`
	Count               int                 `json:"count"`
	Points              []publicMetricPoint `json:"points"`
}

func publicListMetricDefinitions(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}
	defs, err := store.ListMetrics(ctx)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to list metric definitions: "+err.Error(), nil)
	}
	out := make([]metricDefinitionResponse, 0, len(defs))
	for _, def := range defs {
		out = append(out, metricDefinitionResponse{
			Name:          def.Name,
			Description:   metricDescriptionValue(def.Description),
			Type:          string(def.Type),
			Unit:          def.Unit,
			RetentionDays: def.RetentionDays,
			Metadata:      def.Metadata,
			CreatedAt:     def.CreatedAt,
			UpdatedAt:     def.UpdatedAt,
		})
	}
	return out, nil
}

func publicQueryMetrics(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	ctx, release, admissionErr := beginHistoryQuery(ctx)
	if admissionErr != nil {
		return nil, admissionErr
	}
	defer release()
	var params publicMetricQueryParams
	if err := req.BindParams(&params); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}

	metricKeys := normalizeStringList(params.MetricKeys, params.Metrics, []string{params.MetricKey})
	if len(metricKeys) == 0 {
		return nil, rpc.MakeError(rpc.InvalidParams, "metric_keys is required", nil)
	}
	if len(metricKeys) > maxPublicMetricQueryKeys {
		return nil, rpc.MakeError(rpc.InvalidParams, "too many metric keys", nil)
	}

	queryNow := time.Now().UTC()
	start, end, windowErr := historyQueryWindow(firstMetricQueryTime(params.Start, params.StartTime), firstMetricQueryTime(params.End, params.EndTime), params.Hours, 4, queryNow)
	if windowErr != nil {
		return nil, windowErr
	}

	requestedEntityIDs := normalizeStringList(params.EntityIDs, []string{params.EntityID})
	if len(requestedEntityIDs) > maxPublicMetricQueryEntities {
		return nil, rpc.MakeError(rpc.InvalidParams, "too many entities", nil)
	}
	entityIDs, rpcErr := publicMetricEntityIDs(ctx, requestedEntityIDs)
	if rpcErr != nil {
		return nil, rpcErr
	}

	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}

	type metricLoadSpec struct {
		metricKey string
		algorithm metric.Aggregation
		maxPoints int
		interval  time.Duration
	}
	loadSpecs := make([]metricLoadSpec, 0, len(metricKeys))
	for _, metricKey := range metricKeys {
		maxPoints, err := resolveMetricMaxPoints(metricKey, params)
		if err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
		}
		loadSpecs = append(loadSpecs, metricLoadSpec{
			metricKey: metricKey,
			algorithm: resolveMetricAggregation(metricKey, params),
			maxPoints: maxPoints,
		})
	}

	pointLimits := make([]int, len(loadSpecs))
	for i, spec := range loadSpecs {
		pointLimits[i] = spec.maxPoints
	}
	if budgetErr := reserveHistoryShape(len(entityIDs), len(metricKeys), pointLimits); budgetErr != nil {
		return nil, budgetErr
	}

	metricFillEmpty := resolveMetricFillEmpty(params)
	useRaw := publicMetricUsesRawWindow(start, end, queryNow)
	// Raw timestamps have millisecond precision. Only choose the unpaged raw
	// reader when its worst-case points per series fit the requested limit.
	// Otherwise aggregate before loading values; never select or discard samples.
	for _, spec := range loadSpecs {
		if end.Sub(start).Milliseconds()+1 > int64(spec.maxPoints) {
			useRaw = false
		}
	}
	var definitions map[string]metric.Definition
	rawValues := make(map[string][]metric.Point)
	rollupValues := make(map[string]map[metric.Aggregation][]metric.AggregatePoint)
	if len(entityIDs) > 0 && useRaw {
		var err error
		definitions, err = store.GetMetrics(ctx, metricKeys)
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to query metric definitions: "+err.Error(), nil)
		}
		for _, spec := range loadSpecs {
			if _, ok := definitions[spec.metricKey]; !ok {
				return nil, rpc.MakeError(rpc.InvalidParams, "unknown metric key: "+spec.metricKey, nil)
			}
		}
		rawValues, err = store.QueryBatch(ctx, metric.BatchQuery{
			MetricNames: metricKeys,
			EntityIDs:   entityIDs,
			Start:       start,
			End:         end,
			Tags:        params.Tags,
			Order:       metric.OrderAsc,
		})
		if err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, "Failed to query metrics: "+err.Error(), nil)
		}
	} else if len(entityIDs) > 0 {
		batchSpecs := make([]metric.BatchSeriesSpec, 0, len(loadSpecs))
		for i := range loadSpecs {
			loadSpecs[i].interval = metricDownsampleInterval(end.Sub(start), loadSpecs[i].maxPoints)
			loadSpecs[i].interval = store.CompatibleSeriesInterval(start, queryNow, loadSpecs[i].interval)
			batchSpecs = append(batchSpecs, metric.BatchSeriesSpec{
				MetricName:     loadSpecs[i].metricKey,
				Aggregations:   []metric.Aggregation{loadSpecs[i].algorithm},
				Interval:       loadSpecs[i].interval,
				PreserveSeries: true,
			})
		}
		loaded, err := store.SeriesBatch(ctx, metric.BatchSeriesQuery{
			Specs:     batchSpecs,
			EntityIDs: entityIDs,
			Start:     start,
			End:       end,
			Tags:      params.Tags,
			Order:     metric.OrderAsc,
		}, queryNow)
		if err != nil {
			return nil, rpc.MakeError(rpc.InvalidParams, "Failed to query metrics: "+err.Error(), nil)
		}
		definitions = loaded.Definitions
		rollupValues = loaded.Values
		for _, spec := range loadSpecs {
			if _, ok := definitions[spec.metricKey]; !ok {
				return nil, rpc.MakeError(rpc.InvalidParams, "unknown metric key: "+spec.metricKey, nil)
			}
		}
	} else {
		var err error
		definitions, err = store.GetMetrics(ctx, metricKeys)
		if err != nil {
			return nil, rpc.MakeError(rpc.InternalError, "Failed to query metric definitions: "+err.Error(), nil)
		}
		for _, spec := range loadSpecs {
			if _, ok := definitions[spec.metricKey]; !ok {
				return nil, rpc.MakeError(rpc.InvalidParams, "unknown metric key: "+spec.metricKey, nil)
			}
		}
	}

	series := make([]publicMetricSeries, 0, len(metricKeys)*maxInt(1, len(entityIDs)))
	for _, spec := range loadSpecs {
		def := definitions[spec.metricKey]
		item := publicMetricSeries{
			MetricKey:     spec.metricKey,
			Type:          string(def.Type),
			Unit:          def.Unit,
			RetentionDays: def.RetentionDays,
			Tags:          params.Tags,
			FillEmpty:     metricFillEmpty,
			MaxPoints:     spec.maxPoints,
			Downsampled:   !useRaw,
		}
		if useRaw {
			points := rawValues[spec.metricKey]
			item.Points = make([]publicMetricPoint, 0, len(points))
			for _, point := range points {
				item.Points = append(item.Points, publicMetricPoint{
					entityID: point.EntityID,
					Time:     point.Timestamp.UTC(),
					Value:    publicRawMetricValue(point.MetricName, point.Value, metricFillEmpty),
					Count:    1,
					Tags:     point.Tags,
					Labels:   point.Labels,
				})
			}
		} else {
			item.DownsampleAlgorithm = string(spec.algorithm)
			item.IntervalSeconds = spec.interval.Seconds()
			points := rollupValues[spec.metricKey][spec.algorithm]
			item.Points = make([]publicMetricPoint, 0, len(points))
			for _, point := range points {
				item.Points = append(item.Points, publicMetricPoint{
					entityID: point.EntityID,
					Time:     point.Bucket.UTC(),
					Value:    publicRawMetricValue(point.MetricName, point.Value, metricFillEmpty),
					Count:    point.Count,
					Tags:     point.Tags,
				})
			}
		}

		byEntity := make(map[string][]publicMetricSeries, len(entityIDs))
		for _, split := range splitPublicMetricSeries(item) {
			if split.EntityID != "" {
				byEntity[split.EntityID] = append(byEntity[split.EntityID], split)
			}
		}
		for _, entityID := range entityIDs {
			entitySeries := byEntity[entityID]
			if len(entitySeries) == 0 {
				empty := item
				empty.EntityID = entityID
				empty.Count = 0
				empty.Points = nil
				entitySeries = []publicMetricSeries{empty}
			}
			for _, split := range entitySeries {
				if metricFillEmpty {
					split = adaptiveFillPublicMetricSeries(split, start, end)
				}
				series = append(series, split)
			}
		}
	}

	return map[string]any{
		"start":                     start.UTC(),
		"end":                       end.UTC(),
		"server_downsample_default": true,
		"default_points":            defaultMetricQueryPoints,
		"series":                    series,
		"count":                     len(series),
	}, nil
}

func publicMetricUsesRawWindow(start, end, now time.Time) bool {
	retention := metricstore.DefaultRollupRawRetention
	return end.Sub(start) <= retention && end.After(now.UTC().Add(-retention))
}

type publicMetricSeriesGroup struct {
	entityID string
	tagsKey  string
	tags     map[string]string
	points   []publicMetricPoint
}

func splitPublicMetricSeries(base publicMetricSeries) []publicMetricSeries {
	if len(base.Points) == 0 {
		base.Count = 0
		return []publicMetricSeries{base}
	}

	groups := make(map[string]*publicMetricSeriesGroup)
	order := make([]string, 0)
	for _, point := range base.Points {
		entityID := base.EntityID
		if point.entityID != "" {
			entityID = point.entityID
		}
		tags := point.Tags
		point.Tags = tags
		tagsKey := publicMetricTagsKey(tags)
		key := entityID + "\x00" + tagsKey
		group := groups[key]
		if group == nil {
			group = &publicMetricSeriesGroup{
				entityID: entityID,
				tagsKey:  tagsKey,
				tags:     clonePublicMetricTags(tags),
			}
			groups[key] = group
			order = append(order, key)
		}
		group.points = append(group.points, point)
	}

	sort.SliceStable(order, func(i, j int) bool {
		a := groups[order[i]]
		b := groups[order[j]]
		if a.entityID != b.entityID {
			return a.entityID < b.entityID
		}
		return a.tagsKey < b.tagsKey
	})

	out := make([]publicMetricSeries, 0, len(order))
	for _, key := range order {
		group := groups[key]
		item := base
		item.EntityID = group.entityID
		item.Tags = group.tags
		item.Points = group.points
		item.Count = len(group.points)
		out = append(out, item)
	}
	return out
}

// adaptiveFillPublicMetricSeries inserts only the null points needed to mark
// chart boundaries and real collection gaps. The typical collection interval
// is inferred per metric/entity/tag series, so sparse periodic data does not
// expand into hundreds of artificial empty buckets.
func adaptiveFillPublicMetricSeries(series publicMetricSeries, start, end time.Time) publicMetricSeries {
	pointTimes := make([]time.Time, len(series.Points))
	deltas := make([]time.Duration, 0, len(series.Points))
	for i, point := range series.Points {
		pointTimes[i] = point.Time
		if i > 0 {
			delta := point.Time.Sub(pointTimes[i-1])
			if delta > 0 {
				deltas = append(deltas, delta)
			}
		}
	}

	expectedInterval := time.Duration(series.IntervalSeconds * float64(time.Second))
	// Two deltas are the minimum needed to distinguish a regular cadence from
	// one isolated long gap. A lower quartile keeps outages from inflating the
	// inferred cadence when the rest of the series is regular.
	if len(deltas) >= 2 {
		sort.Slice(deltas, func(i, j int) bool { return deltas[i] < deltas[j] })
		observedInterval := deltas[(len(deltas)-1)/4]
		if observedInterval > expectedInterval {
			expectedInterval = observedInterval
		}
	}
	if expectedInterval > 0 {
		series.IntervalSeconds = expectedInterval.Seconds()
	}

	nullPoint := func(at time.Time) publicMetricPoint {
		return publicMetricPoint{
			Time:  at.UTC(),
			Value: nil,
			Tags:  series.Tags,
		}
	}
	filled := make([]publicMetricPoint, 0, len(series.Points)+2)
	if len(pointTimes) == 0 || start.Before(pointTimes[0]) {
		filled = append(filled, nullPoint(start))
	}
	for i, point := range series.Points {
		if i > 0 && expectedInterval > 0 && series.Points[i-1].Value != nil && point.Value != nil {
			delta := pointTimes[i].Sub(pointTimes[i-1])
			if delta > expectedInterval+expectedInterval/2 {
				filled = append(filled, nullPoint(pointTimes[i-1].Add(expectedInterval)))
			}
		}
		filled = append(filled, point)
	}
	// Do not append a trailing null after real data. It would make the final chart
	// bucket blank regardless of whether the tail is a collection delay or a gap.
	if len(pointTimes) == 0 {
		filled = append(filled, nullPoint(end))
	}
	series.Points = filled
	series.Count = len(filled)
	return series
}

func publicMetricTagsKey(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(tags[key])
		b.WriteByte('\x00')
	}
	return b.String()
}

func clonePublicMetricTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		out[key] = value
	}
	return out
}

func publicMetricEntityIDs(ctx context.Context, requested []string) ([]string, *rpc.JsonRpcError) {
	var allClients []models.Client
	err := dbcore.GetDBInstance().WithContext(ctx).Select("uuid", "hidden").Find(&allClients).Error
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to retrieve client information: "+err.Error(), nil)
	}
	isLogin := isLoginFromCtx(ctx)
	hidden := make(map[string]bool, len(allClients))
	visible := make(map[string]bool, len(allClients))
	var allVisible []string
	for _, client := range allClients {
		if client.Hidden {
			hidden[client.UUID] = true
		}
		if client.Hidden && !isLogin {
			continue
		}
		visible[client.UUID] = true
		allVisible = append(allVisible, client.UUID)
	}
	if len(requested) == 0 {
		if len(allVisible) > maxPublicMetricQueryEntities {
			return nil, rpc.MakeError(rpc.InvalidParams, "too many entities after expansion", nil)
		}
		return allVisible, nil
	}
	out := make([]string, 0, len(requested))
	for _, entityID := range requested {
		if hidden[entityID] && !isLogin {
			continue
		}
		if visible[entityID] || !hidden[entityID] {
			out = append(out, entityID)
		}
	}
	return out, nil
}

func normalizeStringList(groups ...[]string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func firstMetricQueryTime(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func metricQueryTimeOrDefault(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback.UTC()
	}
	return value.UTC()
}

func resolveMetricMaxPoints(metricKey string, params publicMetricQueryParams) (int, error) {
	maxPoints := params.MaxPoints
	if maxPoints == 0 {
		maxPoints = defaultMetricQueryPoints
	}
	if v, ok := params.PointsByMetric[metricKey]; ok {
		maxPoints = v
	}
	if v, ok := params.MaxPointsByMetric[metricKey]; ok {
		maxPoints = v
	}
	if maxPoints <= 0 {
		return 0, fmt.Errorf("max points for %s must be a positive integer", metricKey)
	}
	if maxPoints > maxPublicMetricQueryPoints {
		maxPoints = maxPublicMetricQueryPoints
	}
	return maxPoints, nil
}

func resolveMetricAggregation(metricKey string, params publicMetricQueryParams) metric.Aggregation {
	raw := firstNonEmpty(params.Aggregation, params.Algorithm)
	if v := firstNonEmpty(
		params.AggregationByMetric[metricKey],
		params.AlgorithmByMetric[metricKey],
	); v != "" {
		raw = v
	}
	if raw == "" {
		raw = string(metric.AggAvg)
	}
	return metric.Aggregation(normalizeMetricAggregation(raw))
}

func resolveMetricFillEmpty(params publicMetricQueryParams) bool {
	return params.FillEmpty != nil && *params.FillEmpty
}

func publicMetricValue(value float64) *float64 {
	return &value
}

func publicRawMetricValue(metricName string, value float64, fillEmpty bool) *float64 {
	if isNullPingMetricValue(metricName, value, fillEmpty) {
		return nil
	}
	return publicMetricValue(value)
}

func isNullPingMetricValue(metricName string, value float64, fillEmpty bool) bool {
	if !fillEmpty || value != -1 {
		return false
	}
	return metricName == metricstore.MetricPingLatency || metricName == metricstore.MetricPingLoss
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeMetricAggregation(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "average", "mean":
		return string(metric.AggAvg)
	case "std_dev", "stddev_pop", "std_dev_pop":
		return string(metric.AggStdDev)
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func metricDownsampleInterval(rangeDuration time.Duration, maxPoints int) time.Duration {
	if maxPoints <= 0 {
		maxPoints = defaultMetricQueryPoints
	}
	nanos := rangeDuration.Nanoseconds()
	if nanos <= 0 {
		return time.Second
	}
	interval := time.Duration((nanos + int64(maxPoints) - 1) / int64(maxPoints))
	if interval < time.Second {
		return time.Second
	}
	return metric.CeilStandardInterval(interval)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
