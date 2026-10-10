package metric

import (
	"encoding/json"
	"fmt"
	"strings"
)

type seriesDictionaryPlan struct {
	MetricNames []string
	EntityIDs   []string
	Tags        map[string]string
}

type rollupReadPlan struct {
	SeriesIDs    []int64
	ResolutionID int64
	StartMilli   int64
	EndMilli     int64
	Fields       rollupReadFields
}

type rollupReadFields uint16

const (
	rollupReadSum rollupReadFields = 1 << iota
	rollupReadSumSq
	rollupReadMin
	rollupReadMax
	rollupReadFirst
	rollupReadLast
	rollupReadDigest
)

type renderedSQL struct {
	Query string
	Args  []any
}

func (d sqliteDialect) renderSeriesDictionary(tables tables, indexName string, plan seriesDictionaryPlan) renderedSQL {
	return renderExpandedSeriesDictionary(d, tables, " INDEXED BY "+indexName, plan)
}

func renderExpandedSeriesDictionary(d dialect, tables tables, indexHint string, plan seriesDictionaryPlan) renderedSQL {
	args := make([]any, 0, len(plan.MetricNames)+len(plan.EntityIDs)+len(plan.Tags))
	metricPlaceholders := appendPlaceholders(d, &args, plan.MetricNames)
	parts := []string{"s.metric_name IN (" + strings.Join(metricPlaceholders, ", ") + ")"}
	if len(plan.EntityIDs) > 0 {
		entityPlaceholders := appendPlaceholders(d, &args, plan.EntityIDs)
		parts = append(parts, "s.entity_id IN ("+strings.Join(entityPlaceholders, ", ")+")")
	}
	for _, key := range sortedKeys(plan.Tags) {
		args = append(args, plan.Tags[key])
		parts = append(parts, d.jsonExtractEquals("s.tags", key, d.placeholder(len(args))))
	}
	return renderedSQL{
		Query: fmt.Sprintf("SELECT s.id, s.metric_name, s.entity_id, s.tags_hash, s.tags FROM %s s%s WHERE %s ORDER BY s.id ASC", tables.series, indexHint, strings.Join(parts, " AND ")),
		Args:  args,
	}
}

func appendPlaceholders[T ~string](d dialect, args *[]any, values []T) []string {
	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		*args = append(*args, string(value))
		placeholders = append(placeholders, d.placeholder(len(*args)))
	}
	return placeholders
}

// Small selections use the normalized UNIQUE(series_id, resolution_id,
// label_id, bucket_milli) index, not a new write-side index. SQLite names this
// table's sole UNIQUE constraint sqlite_autoindex_<table>_1; schema tests check
// that contract. Broad queries scan the selected time window once instead.
const narrowRollupSeriesLimit = 16

func (d sqliteDialect) renderRollupRead(tables tables, indexName string, plan rollupReadPlan) renderedSQL {
	seriesJSON, _ := json.Marshal(plan.SeriesIDs)
	args := []any{plan.ResolutionID, plan.StartMilli, plan.EndMilli, string(seriesJSON)}
	indexHint := " INDEXED BY " + indexName
	if len(plan.SeriesIDs) <= narrowRollupSeriesLimit {
		indexHint = " INDEXED BY sqlite_autoindex_" + tables.rollups + "_1"
	}
	return renderedSQL{
		Query: fmt.Sprintf("SELECT %s FROM %s r%s WHERE r.resolution_id = %s AND r.bucket_milli >= %s AND r.bucket_milli <= %s AND r.series_id IN (SELECT CAST(value AS INTEGER) FROM json_each(%s)) ORDER BY r.bucket_milli ASC, r.series_id ASC, r.label_id ASC",
			rollupReadColumns(plan.Fields), tables.rollups, indexHint,
			d.placeholder(1), d.placeholder(2), d.placeholder(3), d.placeholder(4)),
		Args: args,
	}
}

func rollupReadColumns(fields rollupReadFields) string {
	columns := []string{"r.series_id", "r.bucket_milli", "r.count"}
	if fields&rollupReadSum != 0 {
		columns = append(columns, "r.sum")
	}
	if fields&rollupReadSumSq != 0 {
		columns = append(columns, "r.sum_sq")
	}
	if fields&rollupReadMin != 0 {
		columns = append(columns, "r.min_val")
	}
	if fields&rollupReadMax != 0 {
		columns = append(columns, "r.max_val")
	}
	if fields&rollupReadFirst != 0 {
		columns = append(columns, "r.first_val", "r.first_ts_milli")
	}
	if fields&rollupReadLast != 0 {
		columns = append(columns, "r.last_val", "r.last_ts_milli")
	}
	if fields&rollupReadDigest != 0 {
		columns = append(columns, "r.digest")
	}
	return strings.Join(columns, ", ")
}
