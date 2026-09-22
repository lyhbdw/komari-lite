package scheduler

import (
	"testing"
	"time"
)

func TestCronScheduleUsesSystemLocalWallClock(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	schedule, err := Parse("0 0 9 * * *")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	after := time.Date(2026, 7, 17, 0, 30, 0, 0, time.UTC)
	want := time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC)
	if got := schedule.Next(after); !got.Equal(want) {
		t.Fatalf("next run = %s, want %s", got, want)
	} else if got.Location() != time.UTC {
		t.Fatalf("next run location = %s, want UTC", got.Location())
	}
}

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

func TestCronDomDowBothRestrictedUsesOR(t *testing.T) {
	// "0 0 9 1 * 1"：每月 1 号 或 每个周一 的 09:00。
	// 2026-07-17 是周五；7 月 1 日是周三，7 月的周一是 6,13,20,27。
	schedule, err := Parse("0 0 9 1 * 1")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	after := time.Date(2026, 7, 17, 0, 30, 0, 0, time.UTC)
	// 下一个匹配：2026-07-20（周一）09:00 本地时间（测试环境 TZ=UTC）。
	want := time.Date(2026, 7, 20, 9, 0, 0, 0, time.Local)
	if got := schedule.Next(after); !got.Equal(want) {
		t.Fatalf("next run = %s, want %s", got, want)
	}
}

func TestCronNextSkipsUnmatchedMonthsQuickly(t *testing.T) {
	// 只在 2 月的 29 日触发的表达式（2027 年 2 月没有 29 日）：
	// Next 必须快速跳过整年而不是逐秒扫描。2028-02-29 是周二。
	schedule, err := Parse("0 0 9 29 2 *")
	if err != nil {
		t.Fatalf("parse schedule: %v", err)
	}
	after := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	want := time.Date(2028, 2, 29, 9, 0, 0, 0, time.Local)
	if got := schedule.Next(after); !got.Equal(want) {
		t.Fatalf("next run = %s, want %s", got, want)
	}
}
