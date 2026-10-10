package jsonrpc

import (
	"context"
	"errors"
	"time"

	"github.com/lyhbdw/komari-lite/pkg/rpc"
)

const (
	maxHistoryOutputPoints = 50000
	maxHistorySeries       = 256
)

var historyQuerySlots = make(chan struct{}, 2)

type historyAdmissionKey struct{}

// beginHistoryQuery is shared by handlers as well as transports, so direct
// internal calls cannot accidentally bypass deadlines or concurrency limits.
func beginHistoryQuery(ctx context.Context) (context.Context, func(), *rpc.JsonRpcError) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := historyContextError(ctx); err != nil {
		return ctx, func() {}, err
	}
	if ctx.Value(historyAdmissionKey{}) != nil {
		return ctx, func() {}, nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	select {
	case historyQuerySlots <- struct{}{}:
		return context.WithValue(queryCtx, historyAdmissionKey{}, true), func() { cancel(); <-historyQuerySlots }, nil
	default:
		cancel()
		return ctx, func() {}, rpc.MakeError(rpc.Unavailable, "too many concurrent history queries", nil)
	}
}

func historyContextError(ctx context.Context) *rpc.JsonRpcError {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return rpc.MakeError(rpc.DeadlineExceeded, "query deadline exceeded", nil)
	case errors.Is(ctx.Err(), context.Canceled):
		return rpc.MakeError(rpc.Cancelled, "query cancelled", nil)
	default:
		return nil
	}
}

// reserveHistoryShape rejects an oversized requested shape before any value
// query. This bounds only the entity/key shape, not tag cardinality: the current
// store API cannot enforce a loading-time series/work budget for arbitrary tags.
func reserveHistoryShape(entities, keys int, points []int) *rpc.JsonRpcError {
	if entities > maxPublicMetricQueryEntities || keys > maxPublicMetricQueryKeys || entities*keys > maxHistorySeries {
		return rpc.MakeError(rpc.InvalidParams, "history series budget exceeded", nil)
	}
	total := int64(0)
	for _, count := range points {
		total += int64(count) * int64(entities)
	}
	if total > maxHistoryOutputPoints {
		return rpc.MakeError(rpc.InvalidParams, "history output budget exceeded", nil)
	}
	return nil
}

// historyQueryWindow applies the same one-year cap to explicit and implicit
// windows. Oversized windows keep the requested end (legacy compatibility).
func historyQueryWindow(startParam, endParam *time.Time, hours float64, defaultHours float64, now time.Time) (time.Time, time.Time, *rpc.JsonRpcError) {
	if hours <= 0 {
		hours = defaultHours
	}
	if hours > maxPublicMetricQueryHours {
		hours = maxPublicMetricQueryHours
	}
	end := metricQueryTimeOrDefault(endParam, now)
	start := metricQueryTimeOrDefault(startParam, end.Add(-time.Duration(hours*float64(time.Hour))))
	if !end.After(start) {
		return time.Time{}, time.Time{}, rpc.MakeError(rpc.InvalidParams, "end must be after start", nil)
	}
	if end.Sub(start) > maxCommonRecordsWindow {
		start = end.Add(-maxCommonRecordsWindow)
	}
	return start, end, nil
}
