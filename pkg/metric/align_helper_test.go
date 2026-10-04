package metric

import (
	"time"
)

// alignTime floors a timestamp to the start of its interval bucket.
//
// alignTime 将时间向下对齐到指定间隔的桶起点。
func alignTime(t time.Time, interval time.Duration) time.Time {
	nano := t.UTC().UnixNano()
	size := interval.Nanoseconds()
	// Floor division toward negative infinity so timestamps before the Unix
	// epoch (negative nanos) align to the bucket start rather than rounding up.
	// Go's % returns a remainder with the dividend's sign, so normalize it.
	rem := ((nano % size) + size) % size
	return time.Unix(0, nano-rem).UTC()
}
