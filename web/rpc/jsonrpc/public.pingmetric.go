package jsonrpc

// public.pingmetric.go
// public:getPingMetricStats —— 三网 ping 统计查询（从 public.metric.go 拆出，
// 与通用指标查询解耦：参数/响应类型、handler 与聚合辅助函数）。

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tumb1er1376/komari-monitor-lite/database/models"
	"github.com/Tumb1er1376/komari-monitor-lite/database/tasks"
	"github.com/Tumb1er1376/komari-monitor-lite/internal/metricstore"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/metric"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/rpc"
)

type publicPingMetricStatsParams struct {
	UUID      string   `json:"uuid"`
	EntityID  string   `json:"entity_id"`
	EntityIDs []string `json:"entity_ids"`

	TaskID  any   `json:"task_id"`
	TaskIDs []any `json:"task_ids"`

	Start     *time.Time `json:"start"`
	StartTime *time.Time `json:"start_time"`
	End       *time.Time `json:"end"`
	EndTime   *time.Time `json:"end_time"`
	Hours     float64    `json:"hours"`

	MaxPoints int `json:"max_points"`
}

type publicPingMetricTaskStats struct {
	EntityID        string            `json:"entity_id"`
	TaskID          string            `json:"task_id"`
	Name            string            `json:"name,omitempty"`
	Type            string            `json:"type,omitempty"`
	Interval        int               `json:"interval,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	Total           int               `json:"total"`
	Valid           int               `json:"valid"`
	Loss            float64           `json:"loss"`
	LossApproximate bool              `json:"loss_approximate,omitempty"`
	Min             *float64          `json:"min,omitempty"`
	Max             *float64          `json:"max,omitempty"`
	Avg             *float64          `json:"avg,omitempty"`
	Latest          *float64          `json:"latest,omitempty"`
	P50             *float64          `json:"p50,omitempty"`
	P99             *float64          `json:"p99,omitempty"`
	StdDev          *float64          `json:"stddev,omitempty"`
	P99P50Ratio     float64           `json:"p99_p50_ratio"`
}

type publicPingMetricStatsResponse struct {
	Start           time.Time                   `json:"start"`
	End             time.Time                   `json:"end"`
	IntervalSeconds float64                     `json:"interval_seconds,omitempty"`
	Stats           []publicPingMetricTaskStats `json:"stats"`
	Count           int                         `json:"count"`
}

func publicGetPingMetricStats(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params publicPingMetricStatsParams
	if err := req.BindParams(&params); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}

	end := metricQueryTimeOrDefault(firstMetricQueryTime(params.End, params.EndTime), time.Now().UTC())
	startFallback := end.Add(-metricQueryHours(params.Hours))
	start := metricQueryTimeOrDefault(firstMetricQueryTime(params.Start, params.StartTime), startFallback)
	if !end.After(start) {
		return nil, rpc.MakeError(rpc.InvalidParams, "end must be after start", nil)
	}

	requestedEntities := normalizeStringList(params.EntityIDs, []string{firstNonEmpty(params.EntityID, params.UUID)})
	if len(requestedEntities) > maxPublicMetricQueryEntities {
		return nil, rpc.MakeError(rpc.InvalidParams, "too many entities", nil)
	}
	entityIDs, rpcErr := publicMetricEntityIDs(ctx, requestedEntities)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if len(entityIDs) == 0 {
		return publicPingMetricStatsResponse{
			Start: start.UTC(),
			End:   end.UTC(),
			Stats: []publicPingMetricTaskStats{},
			Count: 0,
		}, nil
	}

	store := metricstore.GetStore()
	if store == nil {
		return nil, rpc.MakeError(rpc.InternalError, "metric store not initialized", nil)
	}

	taskList, err := tasks.GetAllPingTasks()
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to fetch ping tasks: "+err.Error(), nil)
	}
	taskMap := make(map[string]models.PingTask, len(taskList))
	for _, task := range taskList {
		taskMap[strconv.FormatUint(uint64(task.Id), 10)] = task
	}
	taskFilter := normalizePingMetricTaskIDs(params.TaskID, params.TaskIDs)

	maxPoints := params.MaxPoints
	if maxPoints <= 0 {
		maxPoints = defaultMetricQueryPoints
	}
	now := time.Now().UTC()
	interval := metricDownsampleInterval(end.Sub(start), maxPoints)
	interval = store.CompatibleSeriesInterval(start, now, interval)

	groupsByEntity, err := loadPublicPingMetricAggregateGroups(ctx, store, entityIDs, start, end, interval, now)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to query ping stats: "+err.Error(), nil)
	}
	stats := make([]publicPingMetricTaskStats, 0)
	for _, entityID := range entityIDs {
		stats = append(stats, publicPingStatsFromAggregateGroups(entityID, groupsByEntity[entityID], taskMap, taskFilter)...)
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].EntityID != stats[j].EntityID {
			return stats[i].EntityID < stats[j].EntityID
		}
		return stats[i].TaskID < stats[j].TaskID
	})

	return publicPingMetricStatsResponse{
		Start:           start.UTC(),
		End:             end.UTC(),
		IntervalSeconds: interval.Seconds(),
		Stats:           stats,
		Count:           len(stats),
	}, nil
}

type publicPingMetricAggregateGroups struct {
	Avg           map[string][]metric.AggregatePoint
	Min           map[string][]metric.AggregatePoint
	Max           map[string][]metric.AggregatePoint
	Last          map[string][]metric.AggregatePoint
	P50           map[string][]metric.AggregatePoint
	P99           map[string][]metric.AggregatePoint
	StdDev        map[string][]metric.AggregatePoint
	Loss          map[string][]metric.AggregatePoint
	LossAvailable bool
}

func loadPublicPingMetricAggregateGroups(ctx context.Context, store *metric.Store, entityIDs []string, start, end time.Time, interval time.Duration, now time.Time) (map[string]publicPingMetricAggregateGroups, error) {
	latencyAggregations := []metric.Aggregation{
		metric.AggAvg,
		metric.AggMin,
		metric.AggMax,
		metric.AggLast,
		metric.AggP50,
		metric.AggP99,
		metric.AggStdDev,
	}
	loaded, err := store.SeriesBatch(ctx, metric.BatchSeriesQuery{
		Specs: []metric.BatchSeriesSpec{
			{MetricName: metricstore.MetricPingLatency, Aggregations: latencyAggregations, Interval: interval, PreserveSeries: true},
			{MetricName: metricstore.MetricPingLoss, Aggregations: []metric.Aggregation{metric.AggAvg}, Interval: interval, PreserveSeries: true},
		},
		EntityIDs: entityIDs,
		Start:     start,
		End:       end,
		Order:     metric.OrderAsc,
	}, now)
	if err != nil {
		return nil, err
	}
	latency := loaded.Values[metricstore.MetricPingLatency]
	lossPoints := loaded.Values[metricstore.MetricPingLoss][metric.AggAvg]

	avg := groupPingMetricAggregatePointsByEntity(latency[metric.AggAvg])
	minimum := groupPingMetricAggregatePointsByEntity(latency[metric.AggMin])
	maximum := groupPingMetricAggregatePointsByEntity(latency[metric.AggMax])
	last := groupPingMetricAggregatePointsByEntity(latency[metric.AggLast])
	p50 := groupPingMetricAggregatePointsByEntity(latency[metric.AggP50])
	p99 := groupPingMetricAggregatePointsByEntity(latency[metric.AggP99])
	stddev := groupPingMetricAggregatePointsByEntity(latency[metric.AggStdDev])
	loss := groupPingMetricAggregatePointsByEntity(lossPoints)

	entitySet := make(map[string]struct{})
	for _, groups := range []map[string]map[string][]metric.AggregatePoint{avg, minimum, maximum, last, p50, p99, stddev, loss} {
		for currentEntityID := range groups {
			entitySet[currentEntityID] = struct{}{}
		}
	}
	result := make(map[string]publicPingMetricAggregateGroups, len(entitySet))
	for currentEntityID := range entitySet {
		entityLoss := loss[currentEntityID]
		result[currentEntityID] = publicPingMetricAggregateGroups{
			Avg:           avg[currentEntityID],
			Min:           minimum[currentEntityID],
			Max:           maximum[currentEntityID],
			Last:          last[currentEntityID],
			P50:           p50[currentEntityID],
			P99:           p99[currentEntityID],
			StdDev:        stddev[currentEntityID],
			Loss:          entityLoss,
			LossAvailable: pingMetricGroupsHaveData(entityLoss),
		}
	}
	return result, nil
}

func groupPingMetricAggregatePointsByEntity(points []metric.AggregatePoint) map[string]map[string][]metric.AggregatePoint {
	out := make(map[string]map[string][]metric.AggregatePoint)
	for _, point := range points {
		taskID := strings.TrimSpace(point.Tags["task_id"])
		if taskID == "" {
			continue
		}
		byTask := out[point.EntityID]
		if byTask == nil {
			byTask = make(map[string][]metric.AggregatePoint)
			out[point.EntityID] = byTask
		}
		byTask[taskID] = append(byTask[taskID], point)
	}
	return out
}

func pingMetricGroupsHaveData(groups map[string][]metric.AggregatePoint) bool {
	for _, points := range groups {
		for _, point := range points {
			if point.Count > 0 {
				return true
			}
		}
	}
	return false
}

func publicPingStatsFromAggregateGroups(entityID string, groups publicPingMetricAggregateGroups, taskMap map[string]models.PingTask, taskFilter map[string]bool) []publicPingMetricTaskStats {
	taskIDs := make(map[string]struct{})
	for _, group := range []map[string][]metric.AggregatePoint{
		groups.Avg, groups.Min, groups.Max, groups.Last, groups.P50, groups.P99, groups.StdDev, groups.Loss,
	} {
		for taskID := range group {
			taskIDs[taskID] = struct{}{}
		}
	}

	out := make([]publicPingMetricTaskStats, 0, len(taskIDs))
	for taskID := range taskIDs {
		if len(taskFilter) > 0 && !taskFilter[taskID] {
			continue
		}

		total := aggregatePointCount(groups.Avg[taskID])
		if total == 0 {
			total = aggregatePointCount(groups.Loss[taskID])
		}
		if total == 0 {
			continue
		}

		lossRate, valid, approximate := publicPingLossRate(groups.Avg[taskID], groups.Loss[taskID], total, groups.LossAvailable)
		avg, _ := weightedAggregateValue(groups.Avg[taskID], true)
		p50, _ := weightedAggregateValue(groups.P50[taskID], true)
		p99, _ := weightedAggregateValue(groups.P99[taskID], true)
		stddev, _ := weightedAggregateValue(groups.StdDev[taskID], false)
		minimum := positiveAggregateMin(groups.Min[taskID])
		maximum := positiveAggregateMax(groups.Max[taskID])
		latest := latestPositiveAggregate(groups.Last[taskID])
		if latest == nil {
			latest = latestPositiveAggregate(groups.Avg[taskID])
		}

		stat := publicPingMetricTaskStats{
			EntityID:        entityID,
			TaskID:          taskID,
			Tags:            map[string]string{"task_id": taskID},
			Total:           total,
			Valid:           valid,
			Loss:            lossRate,
			LossApproximate: approximate,
			Min:             minimum,
			Max:             maximum,
			Avg:             avg,
			Latest:          latest,
			P50:             p50,
			P99:             p99,
			StdDev:          stddev,
		}
		if task, ok := taskMap[taskID]; ok {
			stat.Name = task.Name
			stat.Type = task.Type
			stat.Interval = task.Interval
		}
		if p50 != nil && p99 != nil && *p50 > 0 && *p99 >= *p50 {
			adjustedBase := math.Max(math.Min(*p50, 50.0), 10.0)
			stat.P99P50Ratio = (*p99 - *p50) / adjustedBase
		}
		out = append(out, stat)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].TaskID < out[j].TaskID
	})
	return out
}

func normalizePingMetricTaskIDs(taskID any, taskIDs []any) map[string]bool {
	out := make(map[string]bool)
	add := func(value any) {
		switch v := value.(type) {
		case nil:
			return
		case string:
			if raw := strings.TrimSpace(v); raw != "" {
				out[raw] = true
			}
		case float64:
			out[strconv.FormatInt(int64(v), 10)] = true
		case int:
			out[strconv.Itoa(v)] = true
		case int64:
			out[strconv.FormatInt(v, 10)] = true
		case jsonNumber:
			if raw := strings.TrimSpace(v.String()); raw != "" {
				out[raw] = true
			}
		default:
			raw := strings.TrimSpace(fmt.Sprint(v))
			if raw != "" {
				out[raw] = true
			}
		}
	}
	add(taskID)
	for _, value := range taskIDs {
		add(value)
	}
	return out
}

type jsonNumber interface {
	String() string
}

func aggregatePointCount(points []metric.AggregatePoint) int {
	total := 0
	for _, point := range points {
		total += point.Count
	}
	return total
}

func publicPingLossRate(latencyPoints, lossPoints []metric.AggregatePoint, total int, lossAvailable bool) (float64, int, bool) {
	if total <= 0 {
		return 0, 0, !lossAvailable
	}
	if lossAvailable {
		lossCount := 0.0
		for _, point := range lossPoints {
			if point.Count <= 0 {
				continue
			}
			lossCount += math.Max(0, math.Min(1, point.Value)) * float64(point.Count)
		}
		lost := int(math.Round(lossCount))
		if lost > total {
			lost = total
		}
		return lossCount / float64(total) * 100, total - lost, false
	}

	lost := 0
	valid := 0
	for _, point := range latencyPoints {
		if point.Count <= 0 {
			continue
		}
		if point.Value < 0 {
			lost += point.Count
			continue
		}
		valid += point.Count
	}
	return float64(lost) / float64(total) * 100, valid, true
}

func weightedAggregateValue(points []metric.AggregatePoint, skipNegative bool) (*float64, int) {
	sum := 0.0
	count := 0
	for _, point := range points {
		if point.Count <= 0 {
			continue
		}
		if skipNegative && point.Value < 0 {
			continue
		}
		sum += point.Value * float64(point.Count)
		count += point.Count
	}
	if count == 0 {
		return nil, 0
	}
	value := sum / float64(count)
	return &value, count
}

func positiveAggregateMin(points []metric.AggregatePoint) *float64 {
	var out *float64
	for _, point := range points {
		if point.Count <= 0 || point.Value < 0 {
			continue
		}
		value := point.Value
		if out == nil || value < *out {
			out = &value
		}
	}
	return out
}

func positiveAggregateMax(points []metric.AggregatePoint) *float64 {
	var out *float64
	for _, point := range points {
		if point.Count <= 0 || point.Value < 0 {
			continue
		}
		value := point.Value
		if out == nil || value > *out {
			out = &value
		}
	}
	return out
}

func latestPositiveAggregate(points []metric.AggregatePoint) *float64 {
	var out *float64
	var latest time.Time
	for _, point := range points {
		if point.Count <= 0 || point.Value < 0 {
			continue
		}
		if out == nil || point.Bucket.After(latest) {
			value := point.Value
			out = &value
			latest = point.Bucket
		}
	}
	return out
}
