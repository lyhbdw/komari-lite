package attemptlimit

import (
	"testing"
	"time"
)

func TestAllowExpiresBeforeCountingAndNeverDuplicates(t *testing.T) {
	attempts := make(map[string][]time.Time)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		if !Allow(attempts, "client", now, 5*time.Minute, 8, 16) {
			t.Fatalf("attempt %d denied", i+1)
		}
		if got := len(attempts["client"]); got != i+1 {
			t.Fatalf("attempt count = %d, want %d", got, i+1)
		}
	}
	if Allow(attempts, "client", now, 5*time.Minute, 8, 16) {
		t.Fatal("ninth attempt should be denied")
	}
	if !Allow(attempts, "client", now.Add(5*time.Minute), 5*time.Minute, 8, 16) {
		t.Fatal("expired attempts must not block the next window")
	}
}

func TestAllowEvictsOldestKeyAtCapacity(t *testing.T) {
	attempts := make(map[string][]time.Time)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	Allow(attempts, "old", now, time.Minute, 8, 2)
	Allow(attempts, "current", now.Add(time.Second), time.Minute, 8, 2)
	Allow(attempts, "new", now.Add(2*time.Second), time.Minute, 8, 2)
	if len(attempts) != 2 {
		t.Fatalf("tracked keys = %d", len(attempts))
	}
	if _, ok := attempts["old"]; ok {
		t.Fatal("oldest key should have been evicted")
	}
	Allow(attempts, "later", now.Add(2*time.Minute), time.Minute, 8, 2)
	if len(attempts) != 1 {
		t.Fatal("expired keys should be removed")
	}
}
