package migrations

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Legacy row timestamps are offset-free strings; migrateLegacyStream parses
// them back into absolute times when importing old monitoring tables.
//
// 这些格式只出现在旧版数据库里：当前的写入口一律带显式时区偏移，
// 因此解析器保留在这里仅供 legacy_monitoring 迁移使用。

var legacyTimestampLayouts = [...]string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parseLegacyTimestamp(value string, location *time.Location) (time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, fmt.Errorf("timestamp is empty")
	}
	if location == nil {
		location = time.UTC
	}
	if stamp, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return stamp.UTC(), nil
	}
	for _, layout := range legacyTimestampLayouts {
		if stamp, err := time.ParseInLocation(layout, raw, location); err == nil {
			return stamp.UTC(), nil
		}
	}
	epoch, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("unsupported legacy timestamp %q", value)
	}
	return legacyEpochTime(epoch), nil
}

func legacyEpochTime(value int64) time.Time {
	abs := value
	if abs < 0 {
		if abs == math.MinInt64 {
			return time.Unix(0, value).UTC()
		}
		abs = -abs
	}
	switch {
	case abs >= 1e17:
		return time.Unix(0, value).UTC()
	case abs >= 1e14:
		return time.Unix(0, value*int64(time.Microsecond)).UTC()
	case abs >= 1e11:
		return time.UnixMilli(value).UTC()
	default:
		return time.Unix(value, 0).UTC()
	}
}
