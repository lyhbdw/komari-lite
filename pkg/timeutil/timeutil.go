package timeutil

import (
	"time"
)

// SameSystemDate compares calendar dates in the operating system timezone.
func SameSystemDate(left, right time.Time) bool {
	left = left.In(time.Local)
	right = right.In(time.Local)
	ly, lm, ld := left.Date()
	ry, rm, rd := right.Date()
	return ly == ry && lm == rm && ld == rd
}

// FormatSystemDate formats an instant as a system-local calendar date.
func FormatSystemDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.In(time.Local).Format("2006-01-02")
}
