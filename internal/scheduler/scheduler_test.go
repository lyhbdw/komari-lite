package scheduler

import (
	"testing"
	"time"
)

func TestEverySchedulePreservesElapsedDuration(t *testing.T) {
	schedule, err := Parse("@every 90s")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	after := time.Now()
	if got := schedule.Next(after); got.Sub(after) != 90*time.Second {
		t.Fatalf("interval = %s, want 90s", got.Sub(after))
	}
}

func TestParseRejectsCronExpressions(t *testing.T) {
	if _, err := Parse("0 0 9 * * *"); err == nil {
		t.Fatal("cron expressions should no longer be supported")
	}
	if _, err := Parse(""); err == nil {
		t.Fatal("empty spec should be rejected")
	}
	if _, err := Parse("@every 0s"); err == nil {
		t.Fatal("non-positive interval should be rejected")
	}
}
