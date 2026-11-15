package scheduler

import (
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

func clock(hour, minute int) time.Time {
	return time.Date(2026, 11, 11, hour, minute, 0, 0, time.Local)
}

func nineOClock(missed pipeline.Missed) []pipeline.Schedule {
	return []pipeline.Schedule{{Cron: "0 9 * * *", Missed: missed}}
}

func TestTick_FiresWhenTheTimeIsReached(t *testing.T) {
	s := New(0)
	s.Set("nightly", nineOClock(pipeline.MissedSkip), clock(8, 0))
	if got := s.Tick(clock(8, 59)); len(got) != 0 {
		t.Fatalf("fired early: %+v", got)
	}
	got := s.Tick(clock(9, 0).Add(3 * time.Second))
	if len(got) != 1 || got[0].Action != Fire || !got[0].ScheduledFor.Equal(clock(9, 0)) || got[0].Cron != "0 9 * * *" {
		t.Fatalf("decisions = %+v", got)
	}
	if got := s.Tick(clock(9, 1)); len(got) != 0 {
		t.Fatalf("fired twice: %+v", got)
	}
	if next := s.Next("nightly"); len(next) != 1 || !next[0].Equal(clock(9, 0).AddDate(0, 0, 1)) {
		t.Fatalf("next = %v", next)
	}
}

// The laptop was closed at 08:00 and opened at 11:00: the 09:00 run must not start.
func TestTick_SkipsATimeThatPassedWhileNobodyWatched(t *testing.T) {
	s := New(0)
	s.Set("nightly", nineOClock(pipeline.MissedSkip), clock(8, 0))
	got := s.Tick(clock(11, 0))
	if len(got) != 1 || got[0].Action != Missed || !got[0].ScheduledFor.Equal(clock(9, 0)) {
		t.Fatalf("decisions = %+v", got)
	}
	if got := s.Tick(clock(11, 1)); len(got) != 0 {
		t.Fatalf("missed time reported again: %+v", got)
	}
}

func TestTick_CatchesUpOnceWhenTheScheduleAsksForIt(t *testing.T) {
	s := New(0)
	s.Set("sync", []pipeline.Schedule{{Cron: "*/10 * * * *", Missed: pipeline.MissedRun}}, clock(8, 0))
	got := s.Tick(clock(11, 5)) // eighteen times were missed
	if len(got) != 1 || got[0].Action != CatchUp || !got[0].ScheduledFor.Equal(clock(8, 10)) {
		t.Fatalf("decisions = %+v", got)
	}
	if next := s.Next("sync"); !next[0].Equal(clock(11, 10)) {
		t.Fatalf("next = %v", next)
	}
}

func TestTick_GraceBoundary(t *testing.T) {
	s := New(time.Minute)
	s.Set("a", nineOClock(pipeline.MissedSkip), clock(8, 0))
	if got := s.Tick(clock(9, 1)); got[0].Action != Fire {
		t.Fatalf("exactly at the grace limit: %+v", got)
	}
	s.Set("a", nineOClock(pipeline.MissedSkip), clock(8, 0))
	if got := s.Tick(clock(9, 1).Add(time.Second)); got[0].Action != Missed {
		t.Fatalf("past the grace limit: %+v", got)
	}
}

func TestSet_ReplacesAndRemoveForgets(t *testing.T) {
	s := New(0)
	s.Set("a", nineOClock(pipeline.MissedSkip), clock(8, 0))
	s.Set("a", []pipeline.Schedule{{Cron: "30 9 * * *"}}, clock(8, 0))
	if got := s.Tick(clock(9, 0)); len(got) != 0 {
		t.Fatalf("old schedule still fires: %+v", got)
	}
	s.Remove("a")
	if got := s.Tick(clock(9, 30)); len(got) != 0 || len(s.Next("a")) != 0 {
		t.Fatalf("removed pipeline still fires: %+v", got)
	}
	s.Set("b", []pipeline.Schedule{{Cron: "not cron"}}, clock(8, 0))
	if len(s.Next("b")) != 0 {
		t.Fatal("invalid schedule kept")
	}
}

func TestTick_UsesTheScheduleTimezone(t *testing.T) {
	s := New(0)
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	s.Set("tokyo", []pipeline.Schedule{{Cron: "0 9 * * *", Timezone: "Asia/Tokyo"}}, start)
	// 09:00 in Tokyo is 00:00 UTC, so the next one is a day later.
	want := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	if next := s.Next("tokyo"); len(next) != 1 || !next[0].Equal(want) {
		t.Fatalf("next = %v, want %s", next, want)
	}
}
