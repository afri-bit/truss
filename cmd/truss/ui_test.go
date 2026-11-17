package main

import (
	"strings"
	"testing"
	"time"
)

func TestFit(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{{"hello", 10, "hello"}, {"hello world", 8, "hello..."}, {"hello", 3, "hel"}, {"hello", 0, ""}, {"héllo wörld", 8, "héllo..."}} {
		if got := fit(c.in, c.n); got != c.want {
			t.Errorf("fit(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestScreenWidth(t *testing.T) {
	for env, want := range map[string]int{"COLUMNS=100": 100, "COLUMNS=10": minWidth, "COLUMNS=900": maxWidth, "COLUMNS=x": defaultWidth, "": defaultWidth} {
		if got := screenWidth([]string{env}); got != want {
			t.Errorf("%q: %d, want %d", env, got, want)
		}
	}
}

func TestBox_EveryLineHasTheSameWidth(t *testing.T) {
	out := box("A title that is far too long to fit inside the frame of this narrow box", []string{"short", strings.Repeat("long ", 30)}, 40)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %q", lines)
	}
	for _, l := range lines {
		if len(l) != 40 {
			t.Errorf("%q is %d wide", l, len(l))
		}
	}
	if !strings.HasPrefix(lines[0], "+- ") || !strings.HasSuffix(lines[0], "+") || !strings.HasSuffix(lines[1], " |") {
		t.Errorf("frame = %q", lines)
	}
}

func TestBar(t *testing.T) {
	for f, want := range map[float64]string{0: "[....]", 0.5: "[##..]", 1: "[####]", 7: "[####]", -1: "[....]"} {
		if got := bar(f, 4); got != want {
			t.Errorf("bar(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestSpan(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(s int) *time.Time { v := t0.Add(time.Duration(s) * time.Second); return &v }
	end := t0.Add(10 * time.Second)
	cases := []struct {
		start, finish *time.Time
		want          string
	}{
		{at(0), at(5), "|#####.....|"},
		{at(5), at(10), "|.....#####|"},
		{at(2), nil, "|..########|"}, // still running: reaches the end
		{nil, nil, "|..........|"},   // never started
		{at(9), at(9), "|.........#|"},
	}
	for _, c := range cases {
		if got := span(c.start, c.finish, t0, end, 10); got != c.want {
			t.Errorf("span = %q, want %q", got, c.want)
		}
	}
	if got := span(at(0), at(0), t0, t0, 4); got != "|#...|" {
		t.Errorf("instant run = %q", got)
	}
}

func TestAgeAndShort(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	for d, want := range map[time.Duration]string{2 * time.Second: "just now", 45 * time.Second: "45s ago", 5*time.Minute + 10*time.Second: "5m 10s ago", 3*time.Hour + 2*time.Minute: "3h 2m ago", 52 * time.Hour: "2d 4h ago", -90 * time.Second: "in 1m 30s"} {
		if got := age(now, now.Add(-d)); got != want {
			t.Errorf("age(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestElapsedAndBytes(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fin := t0.Add(1500 * time.Millisecond)
	if got := elapsed(&t0, &fin, t0); got != "1.5s" {
		t.Errorf("elapsed = %q", got)
	}
	if got := elapsed(nil, nil, t0); got != "-" {
		t.Errorf("never started = %q", got)
	}
	if got := elapsed(&t0, nil, t0.Add(3*time.Minute)); got != "3m 0s" {
		t.Errorf("running = %q", got)
	}
	for n, want := range map[uint64]string{7: "7B", 2048: "2K", 5 << 20: "5M", 3 << 30: "3.0G"} {
		if got := bytesText(n); got != want {
			t.Errorf("bytesText(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestColumns_AlignAndShortenTheLastColumn(t *testing.T) {
	got := columns([][]string{{"A", "BB", "LAST"}, {"aaa", "b", strings.Repeat("z", 50)}}, 30)
	want := []string{"A    BB  LAST", "aaa  b   " + strings.Repeat("z", 15) + "..."}
	_ = want
	if got[0] != "A    BB  LAST" || len(got[1]) != 30 || !strings.HasSuffix(got[1], "...") {
		t.Fatalf("columns = %q", got)
	}
}
