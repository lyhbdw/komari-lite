package metric

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const recoveryCrashExitCode = 73

// recoveryBase anchors crash-writer fixtures to a bucket inside the 17-minute
// coarse recovery window (5m interval + 12m grace) while ensuring that flush
// targets (base+18m) never cross the 1-hour tier sealing deadline (hour + 1h12m).
// When the standard 5-minute subtraction lands at minute 55 of an hour, adding
// 18m would reach minute 13 of the next hour (past the 12m deadline), sealing
// the 1-hour tier as well and returning 2 instead of 1. Stepping back to minute
// 50 keeps base within the 17m window (at most 14m old) while capping base+18m
// at minute 08 of the next hour.
func recoveryBase() time.Time {
	b := time.Now().UTC().Truncate(5 * time.Minute).Add(-5 * time.Minute)
	if b.Minute() == 55 {
		b = b.Add(-5 * time.Minute)
	}
	return b
}

func TestCrashRecoveryRestoresCommittedMinuteParent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "crash.db")
	base := recoveryBase()
	runRecoveryCrashWriter(t, path, base, "minute")

	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	minutes, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(minutes) != 1 || minutes[0].bucketData.count != 2 || minutes[0].bucketData.sum != 30 {
		t.Fatalf("committed minute after os.Exit = %#v, want count=2 sum=30", minutes)
	}
	if _, err := s.FlushCoarse(ctx, base.Add(18*time.Minute)); err != nil {
		t.Fatal(err)
	}
	parents, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(parents) != 1 || parents[0].bucketData.count != 2 || parents[0].bucketData.sum != 30 {
		t.Fatalf("parent after crash + reopen + seal = %#v, want count=2 sum=30", parents)
	}
}

func TestCrashRecoveryCascadeAndRepeatedReopen(t *testing.T) {
	for _, stage := range []string{"minute", "five-minute", "hour", "mixed"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "cascade.db")
			base := recoveryBase()
			runRecoveryCrashWriter(t, path, base, stage)
			wantCount, wantSum := int64(2), float64(30)
			if stage == "mixed" {
				wantCount, wantSum = 3, 70
			}
			// Close database handles without Store.Close: a second abrupt restart
			// must replay the same committed summaries without double counting.
			for attempt := 0; attempt < 2; attempt++ {
				s, err := Open(ctx, recoveryTestConfig(path))
				if err != nil {
					t.Fatal(err)
				}
				s.closeDBs()
			}
			s, err := Open(ctx, recoveryTestConfig(path))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err := s.FlushCoarse(ctx, base.Truncate(24*time.Hour).Add(24*time.Hour+coarseRollupGrace)); err != nil {
				t.Fatal(err)
			}
			for _, interval := range []time.Duration{5 * time.Minute, time.Hour, 24 * time.Hour} {
				rows, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", interval)
				if err != nil {
					t.Fatal(err)
				}
				var count int64
				var sum float64
				for _, row := range rows {
					count += row.bucketData.count
					sum += row.bucketData.sum
				}
				if count != wantCount || sum != wantSum {
					t.Fatalf("%s after %s crash = count %d sum %g, want %d %g", interval, stage, count, sum, wantCount, wantSum)
				}
			}
			if n, err := s.FlushCoarse(ctx, base.Add(48*time.Hour)); err != nil || n != 0 {
				t.Fatalf("repeat seal = %d, %v", n, err)
			}
		})
	}
}

func TestCrashRecoveryLateMinutePreservesCommittedPrefix(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "late.db")
	base := recoveryBase()
	runRecoveryCrashWriter(t, path, base, "minute")
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	point := Point{MetricName: "crash-recovery", EntityID: "n1", Timestamp: base.Add(30 * time.Second), Value: 40}
	// Only the new sample has an exact timestamp after restart. Replacing it
	// must preserve the committed pre-crash minute contribution (10 + 20).
	writeRollupPointsAt(t, ctx, s, base.Add(6*time.Minute), point)
	point.Value = 50
	writeRollupPointsAt(t, ctx, s, base.Add(6*time.Minute), point)
	if _, err := s.FlushCoarse(ctx, base.Add(18*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, interval := range []time.Duration{time.Minute, 5 * time.Minute} {
		rows, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", interval)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].bucketData.count != 3 || rows[0].bucketData.sum != 80 {
			t.Fatalf("%s late replacement = %#v, want count=3 sum=80", interval, rows)
		}
	}
}

// This helper is executed only in the disposable test subprocess. os.Exit
// deliberately bypasses Store.Close and all deferred shutdown flushes.
func TestCrashRecoveryWriterProcess(t *testing.T) {
	path := os.Getenv("KOMARI_METRIC_CRASH_DB")
	if path == "" {
		return
	}
	baseMilli, err := strconv.ParseInt(os.Getenv("KOMARI_METRIC_CRASH_BASE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	base := time.UnixMilli(baseMilli).UTC()
	ctx := context.Background()
	s, err := Open(ctx, recoveryTestConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateMetric(ctx, Definition{Name: "crash-recovery", RetentionDays: 120}); err != nil {
		t.Fatal(err)
	}
	writeRollupPointsAt(t, ctx, s, base.Add(2*time.Minute),
		Point{MetricName: "crash-recovery", EntityID: "n1", Timestamp: base.Add(10 * time.Second), Value: 10},
		Point{MetricName: "crash-recovery", EntityID: "n1", Timestamp: base.Add(20 * time.Second), Value: 20},
	)
	minutes, err := s.scanRollupRows(ctx, s.reader(), "crash-recovery", time.Minute)
	if err != nil || len(minutes) != 1 || minutes[0].bucketData.count != 2 {
		t.Fatalf("writer did not commit minute: %#v, %v", minutes, err)
	}
	switch os.Getenv("KOMARI_METRIC_CRASH_STAGE") {
	case "tagged":
		writeRollupPointsAt(t, ctx, s, base.Add(2*time.Minute),
			Point{MetricName: "crash-recovery", EntityID: "n2", Timestamp: base.Add(10 * time.Second), Value: 7, Tags: map[string]string{"task": "a"}, Labels: map[string]string{"site": "east"}},
			Point{MetricName: "crash-recovery", EntityID: "n2", Timestamp: base.Add(20 * time.Second), Value: 9, Tags: map[string]string{"task": "b"}, Labels: map[string]string{"site": "west"}},
		)
	case "five-minute":
		if n, err := s.FlushCoarse(ctx, base.Add(18*time.Minute)); err != nil || n != 1 {
			t.Fatalf("commit five-minute = %d, %v", n, err)
		}
	case "hour":
		// Flushing past 1h + grace seals both 5m and 1h tiers. If the fixture
		// runs in the 23:00 UTC hour, adding 1h + grace also crosses midnight
		// (the 24-hour daily boundary), so n may be 2 or 3.
		if n, err := s.FlushCoarse(ctx, base.Add(time.Hour+coarseRollupGrace)); err != nil || (n != 2 && n != 3) {
			t.Fatalf("commit hour = %d, %v", n, err)
		}
	case "mixed":
		if _, err := s.FlushCoarse(ctx, base.Add(18*time.Minute)); err != nil {
			t.Fatal(err)
		}
		// A later minute has committed, while its five-minute and hourly
		// parents remain in memory beside the already committed first child.
		writeRollupPointsAt(t, ctx, s, base.Add(22*time.Minute), Point{
			MetricName: "crash-recovery", EntityID: "n1", Timestamp: base.Add(5*time.Minute + 10*time.Second), Value: 40,
		})
	}
	os.Exit(recoveryCrashExitCode)
}

func recoveryTestConfig(path string) Config {
	return SQLite(path, WithSQLiteProfile(SQLiteProfileDurable))
}

func runRecoveryCrashWriter(t *testing.T, path string, base time.Time, stage string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCrashRecoveryWriterProcess$")
	cmd.Env = append(os.Environ(),
		"KOMARI_METRIC_CRASH_DB="+path,
		"KOMARI_METRIC_CRASH_BASE="+strconv.FormatInt(base.UnixMilli(), 10),
		"KOMARI_METRIC_CRASH_STAGE="+stage,
	)
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != recoveryCrashExitCode {
		t.Fatalf("crash writer exit = %v; want os.Exit(%d)\n%s", err, recoveryCrashExitCode, output)
	}
}
