package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The text screens of the CLI (spec 022) draw with plain ASCII only: no
// colour, no box-drawing characters. They read the same on every terminal,
// in a pipe, and in a log file. Only the live dashboard moves the cursor.

const (
	minWidth     = 60
	maxWidth     = 120
	defaultWidth = 80
)

// screenWidth is the width of the screens: COLUMNS when it is a sensible
// number, else 80. TRUSS does not ask the terminal for its size, which keeps
// the code the same on every operating system.
func screenWidth(env []string) int {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "COLUMNS="); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return min(max(n, minWidth), maxWidth)
			}
		}
	}
	return defaultWidth
}

// fit shortens s to at most n characters, ending in "..." when it is cut.
func fit(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n <= 3 {
		return string([]rune(s)[:n])
	}
	return string([]rune(s)[:n-3]) + "..."
}

// clean makes a string from a file or a daemon safe for a screen: a control
// character (an escape, a bell, a carriage return, a C1 code) becomes "?", so
// a pipeline file cannot move the cursor, retitle the window or hide text on
// the screens that exist to inspect it. JSON output is not cleaned: it escapes.
func clean(s string) string {
	if !strings.ContainsFunc(s, isControl) {
		return s
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case isControl(r):
			return '?'
		}
		return r
	}, s)
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) }

// firstLine is s up to its first line break.
func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimRight(line, "\r")
}

func padRight(s string, n int) string {
	if gap := n - utf8.RuneCountInString(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// columns lays rows out as aligned columns, two spaces apart, and shortens the
// last column so no line is wider than width. The first row is the header.
func columns(rows [][]string, width int) []string {
	if len(rows) == 0 {
		return nil
	}
	n := len(rows[0])
	w := make([]int, n)
	for _, r := range rows {
		for i := 0; i < n && i < len(r); i++ {
			r[i] = clean(r[i])
			w[i] = max(w[i], utf8.RuneCountInString(r[i]))
		}
	}
	fixed := 0
	for _, x := range w[:n-1] {
		fixed += x + 2
	}
	last := max(width-fixed, 8)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		var b strings.Builder
		for i := 0; i < n; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			if i == n-1 {
				b.WriteString(fit(cell, last))
			} else {
				b.WriteString(padRight(cell, w[i]+2))
			}
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

// box draws lines inside a frame of the given outer width, with the title in
// the top edge:
//
//	+- Title -----------+
//	| a line            |
//	+-------------------+
func box(title string, lines []string, width int) string {
	inner := width - 4
	var b strings.Builder
	head := "+- " + fit(clean(title), inner-2) + " "
	b.WriteString(head + strings.Repeat("-", max(width-1-utf8.RuneCountInString(head), 0)) + "+\n")
	for _, l := range lines {
		b.WriteString("| " + padRight(fit(clean(l), inner), inner) + " |\n")
	}
	b.WriteString("+" + strings.Repeat("-", width-2) + "+\n")
	return b.String()
}

// bar draws a meter of the given number of cells: [####......].
// A fraction outside 0 to 1 is clamped.
func bar(fraction float64, cells int) string {
	if fraction < 0 || fraction != fraction {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	full := int(fraction*float64(cells) + 0.5)
	return "[" + strings.Repeat("#", full) + strings.Repeat(".", cells-full) + "]"
}

// span draws when a step ran inside the run as a row of cells:
// |...####...|. A step that has not started is all dots; one that is still
// running reaches to now.
func span(start, finish *time.Time, runStart, runEnd time.Time, cells int) string {
	row := []byte(strings.Repeat(".", cells))
	total := runEnd.Sub(runStart)
	if start != nil && total > 0 {
		end := runEnd
		if finish != nil {
			end = *finish
		}
		from := int(float64(start.Sub(runStart)) / float64(total) * float64(cells))
		to := int(float64(end.Sub(runStart))/float64(total)*float64(cells) + 0.999)
		from = min(max(from, 0), cells-1)
		to = min(max(to, from+1), cells)
		for i := from; i < to; i++ {
			row[i] = '#'
		}
	} else if start != nil {
		row[0] = '#'
	}
	return "|" + string(row) + "|"
}

// age says how long ago t was, in the coarsest useful unit: "just now",
// "5m ago", "3h ago", "2d ago".
func age(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < 0:
		return "in " + short(-d)
	case d < 5*time.Second:
		return "just now"
	}
	return short(d) + " ago"
}

// short writes a duration as its two most significant units: 45s, 5m 10s,
// 3h 2m, 2d 4h.
func short(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

// elapsed is how long a run or step took, or has been running, as a short
// duration; "-" when it never started.
func elapsed(start, finish *time.Time, now time.Time) string {
	if start == nil {
		return "-"
	}
	end := now
	if finish != nil {
		end = *finish
	}
	if end.Before(*start) {
		return "-"
	}
	d := end.Sub(*start)
	if d < 10*time.Second {
		return d.Round(10 * time.Millisecond).String()
	}
	return short(d)
}

// bytesText writes a size as 1.5G, 320M, 12K or 7B.
func bytesText(n uint64) string {
	const unit = 1024
	switch {
	case n >= unit*unit*unit:
		return fmt.Sprintf("%.1fG", float64(n)/(unit*unit*unit))
	case n >= unit*unit:
		return fmt.Sprintf("%.0fM", float64(n)/(unit*unit))
	case n >= unit:
		return fmt.Sprintf("%.0fK", float64(n)/unit)
	}
	return fmt.Sprintf("%dB", n)
}
