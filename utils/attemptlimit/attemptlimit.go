// Package attemptlimit shares bounded sliding-window attempt accounting.
package attemptlimit

import "time"

// Allow records one allowed attempt, pruning expired entries first. The caller
// must hold the mutex protecting attempts; no goroutines or global state are
// created by the limiter. At capacity the least-recently-used client is evicted.
func Allow(attempts map[string][]time.Time, key string, now time.Time, window time.Duration, maxAttempts, maxKeys int) bool {
	if maxAttempts <= 0 || maxKeys <= 0 || window <= 0 {
		return false
	}
	cutoff := now.Add(-window)
	for existingKey, oldEntries := range attempts {
		entries := oldEntries[:0]
		for _, at := range oldEntries {
			if at.After(cutoff) {
				entries = append(entries, at)
			}
		}
		clear(oldEntries[len(entries):])
		if len(entries) == 0 {
			delete(attempts, existingKey)
		} else {
			attempts[existingKey] = entries
		}
	}
	entries := attempts[key]
	if len(entries) >= maxAttempts {
		return false
	}
	if len(entries) == 0 && len(attempts) >= maxKeys {
		oldestKey := ""
		var oldest time.Time
		found := false
		for candidate, candidateEntries := range attempts {
			last := candidateEntries[len(candidateEntries)-1]
			if !found || last.Before(oldest) || (last.Equal(oldest) && candidate < oldestKey) {
				oldestKey, oldest, found = candidate, last, true
			}
		}
		delete(attempts, oldestKey)
	}
	attempts[key] = append(entries, now)
	return true
}
