package metricstore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Tumb1er1376/komari-monitor-lite/database/models"
	"github.com/Tumb1er1376/komari-monitor-lite/pkg/metric"
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

	interval := recordSeriesInterval(s, start, end, time.Now().UTC())
	entityIDs, err := listRecordEntityIDs(ctx, s, start, end, interval)
	if err != nil {
		return nil, err
	}
	var records []models.Record
	for _, entityID := range entityIDs {
		items, err := getRecordsByClientAndTimeFromSeries(ctx, s, entityID, start, end)
		if err != nil {
			return nil, err
		}
		records = append(records, items...)
	}
	sortRecords(records)
	return records, nil
}

type recordSeriesKey struct {
	client string
	ts     int64
}

func getRecordsByClientAndTimeFromSeries(ctx context.Context, s *metric.Store, clientUUID string, start, end time.Time) ([]models.Record, error) {
	now := time.Now().UTC()
	interval := recordSeriesInterval(s, start, end, now)
	recordMap := make(map[recordSeriesKey]*models.Record)

	for _, metricName := range loadRecordMetricNames {
		points, err := s.Series(ctx, metric.AggregateQuery{
			Query: metric.Query{
				MetricName: metricName,
				EntityID:   clientUUID,
				Start:      start,
				End:        end,
				Order:      metric.OrderAsc,
			},
			Aggregation: recordMetricAggregation(metricName),
			Interval:    interval,
		}, now)
		if err != nil {
			return nil, fmt.Errorf("failed to query metric %s: %w", metricName, err)
		}
		for _, point := range points {
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

func listRecordEntityIDs(ctx context.Context, s *metric.Store, start, end time.Time, interval time.Duration) ([]string, error) {
	seen := make(map[string]struct{})
	for _, metricName := range loadRecordMetricNames {
		ids, err := s.EntityIDs(ctx, metric.Query{
			MetricName: metricName,
			Start:      start.Add(-interval),
			End:        end,
		})
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
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
