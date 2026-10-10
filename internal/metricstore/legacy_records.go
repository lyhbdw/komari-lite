package metricstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/pkg/metric"
)

// GetRecordsByClientAndTime 从 metric store 查询记录并重构为 models.Record
func GetRecordsByClientAndTime(ctx context.Context, clientUUID string, start, end time.Time) ([]models.Record, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	return getRecordsByClientAndTimeFromSeries(ctx, s, clientUUID, start, end)
}

// GetRecordsByTime 从 metric store 查询所有客户端在时间范围内的记录
func GetRecordsByTime(ctx context.Context, start, end time.Time) ([]models.Record, error) {
	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	return getRecordsByClientAndTimeFromSeries(ctx, s, "", start, end)
}

type recordSeriesKey struct {
	client string
	ts     int64
}

func getRecordsByClientAndTimeFromSeries(ctx context.Context, s *metric.Store, clientUUID string, start, end time.Time) ([]models.Record, error) {
	now := time.Now().UTC()
	interval := recordSeriesInterval(s, start, end, now)
	recordMap := make(map[recordSeriesKey]*models.Record)

	specs := make([]metric.BatchSeriesSpec, 0, len(loadRecordMetricNames))
	for _, metricName := range loadRecordMetricNames {
		specs = append(specs, metric.BatchSeriesSpec{
			MetricName:     metricName,
			Aggregations:   []metric.Aggregation{recordMetricAggregation(metricName)},
			Interval:       interval,
			PreserveSeries: true, // Never merge different nodes (or GPU tag series).
		})
	}
	var entityIDs []string
	if clientUUID != "" {
		entityIDs = []string{clientUUID}
	}
	result, err := s.SeriesBatch(ctx, metric.BatchSeriesQuery{
		Specs: specs, EntityIDs: entityIDs, Start: start, End: end, Order: metric.OrderAsc,
	}, now)
	if err != nil {
		return nil, fmt.Errorf("failed to query record metrics: %w", err)
	}
	for _, metricName := range loadRecordMetricNames {
		for _, point := range result.Values[metricName][recordMetricAggregation(metricName)] {
			entityID := point.EntityID
			if entityID == "" {
				entityID = clientUUID
			}
			key := recordSeriesKey{client: entityID, ts: point.Bucket.Unix()}
			if recordMap[key] == nil {
				recordMap[key] = &models.Record{
					Client: entityID,
					Time:   point.Bucket.UTC(),
				}
			}
			applyRecordMetricValue(recordMap[key], metricName, point.Value)
		}
	}

	records := make([]models.Record, 0, len(recordMap))
	for _, rec := range recordMap {
		records = append(records, *rec)
	}
	sortRecords(records)
	return records, nil
}

func recordMetricAggregation(metricName string) metric.Aggregation {
	switch metricName {
	case MetricTrafficUp, MetricTrafficDown:
		return metric.AggSum
	case MetricNetTotalUp, MetricNetTotalDown:
		return metric.AggLast
	default:
		return metric.AggAvg
	}
}

func recordSeriesInterval(s *metric.Store, start, end, now time.Time) time.Duration {
	interval := recordDownsampleInterval(end.Sub(start), 500)
	return s.CompatibleSeriesInterval(start, now, interval)
}

func recordDownsampleInterval(rangeDuration time.Duration, maxPoints int) time.Duration {
	if maxPoints <= 0 {
		maxPoints = 500
	}
	nanos := rangeDuration.Nanoseconds()
	if nanos <= 0 {
		return time.Second
	}
	interval := time.Duration((nanos + int64(maxPoints) - 1) / int64(maxPoints))
	if interval < time.Second {
		return time.Second
	}
	return metric.FloorStandardInterval(interval)
}

func applyRecordMetricValue(rec *models.Record, metricName string, value float64) {
	switch metricName {
	case MetricCPU:
		rec.Cpu = float32(value)
	case MetricGPU:
		rec.Gpu = float32(value)
	case MetricRAM:
		rec.Ram = int64(value)
	case MetricSwap:
		rec.Swap = int64(value)
	case MetricLoad:
		rec.Load = float32(value)
	case MetricDisk:
		rec.Disk = int64(value)
	case MetricNetIn:
		rec.NetIn = int64(value)
	case MetricNetOut:
		rec.NetOut = int64(value)
	case MetricNetTotalUp:
		rec.NetTotalUp = int64(value)
	case MetricNetTotalDown:
		rec.NetTotalDown = int64(value)
	case MetricTrafficUp:
		rec.TrafficUp = int64(value)
	case MetricTrafficDown:
		rec.TrafficDown = int64(value)
	case MetricProcess:
		rec.Process = int(value)
	case MetricConnections:
		rec.Connections = int(value)
	case MetricConnectionsUDP:
		rec.ConnectionsUdp = int(value)
	}
}

func sortRecords(records []models.Record) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Client != records[j].Client {
			return records[i].Client < records[j].Client
		}
		return records[i].Time.Before(records[j].Time)
	})
}

// GetPingRecords 从 metric store 查询兼容旧接口的 ping 记录。
//
// 旧接口过去直接读取 ping_records。这里使用与 queryMetrics 相同的 Series
