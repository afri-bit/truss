package cron

import (
	"strings"
	"testing"
	"time"
)

func at(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNext(t *testing.T) {
	tests := []struct {
		expr, from, want string
	}{
		{"* * * * *", "2026-11-11 10:00", "2026-11-11 10:01"},
		{"0 9 * * *", "2026-11-11 08:59", "2026-11-11 09:00"},
		{"0 9 * * *", "2026-11-11 09:00", "2026-11-12 09:00"},
		{"*/15 * * * *", "2026-11-11 10:46", "2026-11-11 11:00"},
		{"0 9 * * 1-5", "2026-11-13 09:00", "2026-11-16 09:00"}, // Friday to Monday
		{"0 9 * * mon-fri", "2026-11-13 09:00", "2026-11-16 09:00"},
		{"30 18 1 * *", "2026-11-11 00:00", "2026-12-01 18:30"},
		{"0 0 1 jan *", "2026-11-11 00:00", "2027-01-01 00:00"},
		{"0 0 29 2 *", "2026-11-11 00:00", "2028-02-29 00:00"},
		{"0 12 * * 7", "2026-11-11 00:00", "2026-11-15 12:00"},   // 7 is Sunday
		{"0 0 13 * 5", "2026-11-11 00:00", "2026-11-13 00:00"},   // 13th or a Friday
		{"0 0 13 * 5", "2026-11-13 00:00", "2026-11-20 00:00"},   // next Friday
		{"5/20 * * * *", "2026-11-11 10:26", "2026-11-11 10:45"}, // 5, 25, 45
		{"0 8,17 * * *", "2026-11-11 09:00", "2026-11-11 17:00"},
		{"@daily", "2026-11-11 10:00", "2026-11-12 00:00"},
		{"@hourly", "2026-11-11 10:00", "2026-11-11 11:00"},
		{"@weekly", "2026-11-11 10:00", "2026-11-15 00:00"},
	}
	for _, tc := range tests {
		e, err := Parse(tc.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.expr, err)
			continue
		}
		if got, want := e.Next(at(t, tc.from)), at(t, tc.want); !got.Equal(want) {
			t.Errorf("%q after %s = %s, want %s", tc.expr, tc.from, got.Format("2006-01-02 15:04"), tc.want)
		}
	}
}

func TestNextIsStrictlyAfterAndMatches(t *testing.T) {
	e, err := Parse("*/7 3-5 * * *")
	if err != nil {
		t.Fatal(err)
	}
	now := at(t, "2026-11-11 00:00")
	for range 50 {
		next := e.Next(now)
		if !next.After(now) || !e.Matches(next) {
			t.Fatalf("Next(%s) = %s", now, next)
		}
		now = next
	}
}

func TestNextUsesTheLocationOfItsArgument(t *testing.T) {
	loc := time.FixedZone("plus2", 2*60*60)
	e, _ := Parse("0 9 * * *")
	got := e.Next(time.Date(2026, 11, 11, 8, 0, 0, 0, loc))
	if want := time.Date(2026, 11, 11, 9, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestParseRejects(t *testing.T) {
	tests := map[string]string{
		"":             "want 5 fields",
		"* * * *":      "want 5 fields",
		"60 * * * *":   "minute",
		"* 24 * * *":   "hour",
		"* * 0 * *":    "day of month",
		"* * * 13 *":   "month",
		"* * * * 8":    "day of week",
		"*/0 * * * *":  "bad step",
		"5-1 * * * *":  "backwards",
		"a * * * *":    "minute",
		"0 0 31 2 *":   "no date ever matches",
		"@sometimes":   "want 5 fields",
		"1,,2 * * * *": "minute",
	}
	for expr, want := range tests {
		_, err := Parse(expr)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) error = %v, want it to mention %q", expr, err, want)
		}
	}
}

func TestZeroValueNeverFires(t *testing.T) {
	var e Expr
	if !e.Next(time.Now()).IsZero() {
		t.Fatal("zero Expr returned a time")
	}
}
