package metric

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// scanRollupRowsBetween is a test helper for inspecting rollup tier rows.
func (s *Store) scanRollupRowsBetween(ctx context.Context, metricName, entityID string, tags map[string]string, resolutionMilli, lowerBucket, upperBucket int64, needDigest bool) ([]storedRollup, error) {
	return s.scanRollupRowsBetweenWith(ctx, s.reader(), metricName, entityID, tags, time.Duration(resolutionMilli)*time.Millisecond, lowerBucket, upperBucket, needDigest)
}

func (s *Store) scanRollupRowsBetweenWith(ctx context.Context, q querier, metricName, entityID string, tags map[string]string, resolution time.Duration, lowerBucket, upperBucket int64, needDigest bool) ([]storedRollup, error) {
	args := []any{metricName, resolution.Milliseconds(), lowerBucket, upperBucket}
	parts := []string{
		"s.metric_name = " + s.dialect.placeholder(1),
		"d.resolution_milli = " + s.dialect.placeholder(2),
		"r.bucket_milli >= " + s.dialect.placeholder(3),
		"r.bucket_milli <= " + s.dialect.placeholder(4),
	}
	if entityID != "" {
		args = append(args, entityID)
		parts = append(parts, "s.entity_id = "+s.dialect.placeholder(len(args)))
	}
	for _, key := range sortedKeys(tags) {
		args = append(args, tags[key])
		parts = append(parts, s.dialect.jsonExtractEquals("s.tags", key, s.dialect.placeholder(len(args))))
	}
	columns := "s.entity_id, s.tags_hash, s.tags, l.labels_hash, l.labels, r.bucket_milli, r.count, r.sum, r.sum_sq, r.min_val, r.max_val, r.first_val, r.first_ts_milli, r.last_val, r.last_ts_milli"
	if needDigest {
		columns += ", r.digest"
	}
	sqlText := fmt.Sprintf("SELECT %s FROM %s r JOIN %s s ON s.id = r.series_id JOIN %s d ON d.id = r.resolution_id JOIN %s l ON l.id = r.label_id WHERE %s ORDER BY r.bucket_milli ASC", columns, s.tables.rollups, s.tables.series, s.tables.resolutions, s.tables.labels, strings.Join(parts, " AND "))
	rows, err := q.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStoredRollupsForMaintenance(rows, needDigest, s.cfg.RollupPolicy.compression())
}

func scanStoredRollupsForMaintenance(rows *sql.Rows, needDigest bool, compression float64) ([]storedRollup, error) {
	result := make([]storedRollup, 0)
	for rows.Next() {
		var entityID, tagsHash, tagsJSON, labelsHash, labelsJSON string
		var bucket, count, firstTS, lastTS int64
		var sum, sumSq, min, max, firstVal, lastVal float64
		var digest []byte
		var err error
		if needDigest {
			err = rows.Scan(&entityID, &tagsHash, &tagsJSON, &labelsHash, &labelsJSON, &bucket, &count, &sum, &sumSq, &min, &max, &firstVal, &firstTS, &lastVal, &lastTS, &digest)
		} else {
			err = rows.Scan(&entityID, &tagsHash, &tagsJSON, &labelsHash, &labelsJSON, &bucket, &count, &sum, &sumSq, &min, &max, &firstVal, &firstTS, &lastVal, &lastTS)
		}
		if err != nil {
			return nil, err
		}
		var decoded *TDigest
		if needDigest {
			decoded, err = digestFromRollup(count, min, max, digest, compression)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, storedRollup{entityID: entityID, bucket: bucket, bucketData: &rollupBucket{
			count: count, sum: sum, sumSq: sumSq, min: min, max: max,
			firstVal: firstVal, firstTS: firstTS, lastVal: lastVal, lastTS: lastTS,
			digest: decoded, tagsHash: tagsHash, tagsJSON: tagsJSON, labelsHash: labelsHash, labelsJSON: labelsJSON,
		}})
	}
	return result, rows.Err()
}

func (s *Store) writeRollupBucketsTx(ctx context.Context, metricName string, interval time.Duration, buckets map[rollupKey]*rollupBucket, tx *sql.Tx) (int, error) {
	return s.mergeRollupBucketsWithDictionaryTx(ctx, metricName, interval, buckets, newRollupDictionaryCache(), tx)
}

func (s *Store) scanRollupRows(ctx context.Context, q querier, metricName string, interval time.Duration) ([]storedRollup, error) {
	return s.scanRollupRowsBetweenWith(ctx, q, metricName, "", nil, interval, -1<<62, 1<<62, true)
}

func percentileSorted(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}
	rank := p * float64(len(values)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(values) {
		return values[len(values)-1]
	}
	frac := rank - float64(lo)
	return values[lo]*(1-frac) + values[hi]*frac
}
