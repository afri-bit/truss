package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Expr is a parsed cron expression. The zero value matches nothing.
type Expr struct {
	minute, hour, dom, month, dow uint64
	// domAny and dowAny record a "*" in the day fields. When both day fields
	// are restricted a day matches if either one does, as in classic cron.
	domAny, dowAny bool
}

type field struct {
	name     string
	min, max int
	names    []string
}

var (
	monthNames = []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
	dayNames   = []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

	fields = [5]field{
		{name: "minute", min: 0, max: 59},
		{name: "hour", min: 0, max: 23},
		{name: "day of month", min: 1, max: 31},
		{name: "month", min: 1, max: 12, names: monthNames},
		{name: "day of week", min: 0, max: 7, names: dayNames},
	}

	shortcuts = map[string]string{
		"@hourly":   "0 * * * *",
		"@daily":    "0 0 * * *",
		"@midnight": "0 0 * * *",
		"@weekly":   "0 0 * * 0",
		"@monthly":  "0 0 1 * *",
		"@yearly":   "0 0 1 1 *",
		"@annually": "0 0 1 1 *",
	}
)

// Parse reads a five-field cron expression or an @-shortcut.
func Parse(expr string) (Expr, error) {
	text := strings.TrimSpace(expr)
	if long, ok := shortcuts[strings.ToLower(text)]; ok {
		text = long
	}
	parts := strings.Fields(text)
	if len(parts) != len(fields) {
		return Expr{}, fmt.Errorf("cron %q: want 5 fields (minute hour day-of-month month day-of-week), got %d", expr, len(parts))
	}
	var sets [5]uint64
	for i, part := range parts {
		set, err := parseField(part, fields[i])
		if err != nil {
			return Expr{}, fmt.Errorf("cron %q: %w", expr, err)
		}
		sets[i] = set
	}
	// Sunday may be written 0 or 7.
	if sets[4]&(1<<7) != 0 {
		sets[4] = sets[4]&^(1<<7) | 1
	}
	e := Expr{
		minute: sets[0], hour: sets[1], dom: sets[2], month: sets[3], dow: sets[4],
		domAny: strings.HasPrefix(parts[2], "*"),
		dowAny: strings.HasPrefix(parts[4], "*"),
	}
	if !e.possible() {
		return Expr{}, fmt.Errorf("cron %q: no date ever matches", expr)
	}
	return e, nil
}

// parseField turns one field, such as "1-5", "*/15" or "mon,wed", into a bit set.
func parseField(text string, f field) (uint64, error) {
	var set uint64
	for _, item := range strings.Split(text, ",") {
		rangeText, stepText, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return 0, fmt.Errorf("%s: bad step in %q", f.name, item)
			}
			step = n
		}
		lo, hi := f.min, f.max
		if rangeText != "*" {
			loText, hiText, isRange := strings.Cut(rangeText, "-")
			var err error
			if lo, err = parseValue(loText, f); err != nil {
				return 0, err
			}
			hi = lo
			if isRange {
				if hi, err = parseValue(hiText, f); err != nil {
					return 0, err
				}
			} else if hasStep {
				hi = f.max // "5/10" means from 5 to the end, every 10
			}
			if lo > hi {
				return 0, fmt.Errorf("%s: range %q is backwards", f.name, rangeText)
			}
		} else if f.max == 7 {
			hi = 6 // "*" in day of week covers each day once
		}
		for v := lo; v <= hi; v += step {
			set |= 1 << uint(v)
		}
	}
	return set, nil
}

func parseValue(text string, f field) (int, error) {
	for i, name := range f.names {
		if strings.EqualFold(text, name) {
			return i + f.min, nil
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < f.min || n > f.max {
		return 0, fmt.Errorf("%s: %q is not a value from %d to %d", f.name, text, f.min, f.max)
	}
	return n, nil
}

// possible reports whether some date can match, which rules out
// expressions such as "0 0 31 2 *" (February 31st).
func (e Expr) possible() bool {
	if !e.dowAny || e.domAny {
		return true
	}
	longest := [13]int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for m := 1; m <= 12; m++ {
		if e.month&(1<<uint(m)) == 0 {
			continue
		}
		for d := 1; d <= longest[m]; d++ {
			if e.dom&(1<<uint(d)) != 0 {
				return true
			}
		}
	}
	return false
}

func (e Expr) dayMatches(t time.Time) bool {
	dom := e.dom&(1<<uint(t.Day())) != 0
	dow := e.dow&(1<<uint(t.Weekday())) != 0
	switch {
	case e.domAny && e.dowAny:
		return true
	case e.domAny:
		return dow
	case e.dowAny:
		return dom
	default:
		return dom || dow
	}
}

// Matches reports whether t, read in its own location, is a scheduled minute.
func (e Expr) Matches(t time.Time) bool {
	return e.minute&(1<<uint(t.Minute())) != 0 &&
		e.hour&(1<<uint(t.Hour())) != 0 &&
		e.month&(1<<uint(t.Month())) != 0 &&
		e.dayMatches(t)
}

// Next returns the first scheduled minute strictly after t, in t's location.
// It returns the zero time when the expression is the zero value.
//
// Wall-clock times that a daylight-saving change skips do not occur, so a
// schedule inside the skipped hour does not fire that day. A repeated hour
// fires on its first occurrence only when the schedule names a single minute
// of it; this matches common cron behavior closely enough for a local tool.
func (e Expr) Next(t time.Time) time.Time {
	if e.minute == 0 {
		return time.Time{}
	}
	loc := t.Location()
	t = t.Truncate(time.Minute).Add(time.Minute)
	limit := t.AddDate(8, 0, 0) // a leap day on a given weekday recurs within 28 years; 8 covers real schedules
	for t.Before(limit) {
		switch {
		case e.month&(1<<uint(t.Month())) == 0:
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
		case !e.dayMatches(t):
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
		case e.hour&(1<<uint(t.Hour())) == 0:
			next := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, loc)
			if !next.After(t) { // a clock change can make the wall clock stand still
				next = t.Truncate(time.Hour).Add(time.Hour)
			}
			t = next
		case e.minute&(1<<uint(t.Minute())) == 0:
			t = t.Add(time.Minute)
		default:
			return t
		}
	}
	return time.Time{}
}
