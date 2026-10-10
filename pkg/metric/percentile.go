package metric

import (
	"strconv"
)

// Pxx builds the Aggregation for an arbitrary percentile. The argument is a
// percentage in (0,100): Pxx(99.9) -> "p99.9", Pxx(50) -> "p50". The fixed
// AggP50/AggP95/AggP99 constants are the common cases.
//
// Pxx 根据任意百分位构造 Aggregation。参数为 (0,100) 内的百分比：
// Pxx(99.9) -> "p99.9"，Pxx(50) -> "p50"。固定的 AggP50/AggP95/AggP99
// 是常用特例。
func Pxx(p float64) Aggregation {
	// Trim trailing zeros so Pxx(95) == AggP95 ("p95"), not "p95.000000".
	s := strconv.FormatFloat(p, 'f', -1, 64)
	return Aggregation("p" + s)
}

// parsePercentile reports whether agg names a percentile and, if so, returns
// the corresponding fraction in [0,1]. "p99.9" -> 0.999. Out-of-range
// percentages (<=0 or >=100) are rejected so validation can reject them.
//
// parsePercentile 判断 agg 是否命名了百分位；如果是，则返回对应的 [0,1] 小数。
// 例如 "p99.9" -> 0.999。越界百分比（<=0 或 >=100）会被拒绝，以便校验逻辑
// 能拒绝它们。
func parsePercentile(agg Aggregation) (float64, bool) {
	s := string(agg)
	if len(s) < 2 || (s[0] != 'p' && s[0] != 'P') {
		return 0, false
	}
	pct, err := strconv.ParseFloat(s[1:], 64)
	if err != nil {
		return 0, false
	}
	if pct <= 0 || pct >= 100 {
		return 0, false
	}
	return pct / 100, true
}

// isPercentile reports whether agg is any percentile aggregation.
//
// isPercentile 判断聚合类型是否为任意百分位聚合。
func isPercentile(agg Aggregation) bool {
	_, ok := parsePercentile(agg)
	return ok
}
