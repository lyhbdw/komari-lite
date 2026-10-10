package api

import (
	"fmt"
	"testing"
	"time"
)

func resetSetupAttempts(t *testing.T) {
	t.Helper()
	setupLimiter.Lock()
	previous := setupLimiter.attempts
	setupLimiter.attempts = make(map[string][]time.Time)
	setupLimiter.Unlock()
	t.Cleanup(func() {
		setupLimiter.Lock()
		setupLimiter.attempts = previous
		setupLimiter.Unlock()
	})
}

func TestSetupLimiterCountsEachAttemptOnce(t *testing.T) {
	resetSetupAttempts(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		if !setupAllowed("client", now.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("attempt %d rejected before the eight-attempt limit", i+1)
		}
		setupLimiter.Lock()
		count := len(setupLimiter.attempts["client"])
		setupLimiter.Unlock()
		if count != i+1 {
			t.Fatalf("%d calls stored %d attempts", i+1, count)
		}
	}
	if setupAllowed("client", now.Add(8*time.Second)) {
		t.Fatal("ninth attempt should be rejected")
	}
}

func TestSetupLimiterExpiresBlockedClient(t *testing.T) {
	resetSetupAttempts(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	setupLimiter.Lock()
	setupLimiter.attempts["client"] = []time.Time{now, now, now, now, now, now, now, now}
	setupLimiter.Unlock()
	if !setupAllowed("client", now.Add(5*time.Minute)) {
		t.Fatal("attempts on the window boundary must expire")
	}
}

func TestSetupLimiterBoundsTrackedClients(t *testing.T) {
	resetSetupAttempts(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4096+8; i++ {
		if !setupAllowed(fmt.Sprintf("client-%d", i), now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatal("a new client should not inherit another client's limit")
		}
	}
	setupLimiter.Lock()
	count := len(setupLimiter.attempts)
	setupLimiter.Unlock()
	if count > 4096 {
		t.Fatalf("limiter retains %d clients, want at most 4096", count)
	}
	if !setupAllowed("later", now.Add(6*time.Minute)) {
		t.Fatal("new window should allow attempts")
	}
	setupLimiter.Lock()
	count = len(setupLimiter.attempts)
	setupLimiter.Unlock()
	if count != 1 {
		t.Fatalf("expired clients were not pruned: %d remain", count)
	}
}
