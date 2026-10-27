package runlog

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSetup_FiveLinesOmitEnvironment(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, clockAt(time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)))
	log.Setup("macOS", "arm64", "/bin/sh", "/work")

	lines := splitLines(t, buf.String())
	if len(lines) != 5 {
		t.Fatalf("setup lines = %d\n%s", len(lines), buf.String())
	}
	want := []string{
		"2026-10-26 10:00:00 setup environment ready",
		"2026-10-26 10:00:00 setup os macOS",
		"2026-10-26 10:00:00 setup arch arm64",
		"2026-10-26 10:00:00 setup shell /bin/sh",
		"2026-10-26 10:00:00 setup directory /work",
	}
	for i, line := range want {
		if lines[i] != line {
			t.Fatalf("line %d = %q, want %q", i, lines[i], line)
		}
	}
	for _, secret := range []string{"TRUSS_SECRET", "SECRET_VALUE", "PATH="} {
		if strings.Contains(buf.String(), secret) {
			t.Fatalf("setup lists %q:\n%s", secret, buf.String())
		}
	}
}

func TestTimestamp_DoesNotMoveBackward(t *testing.T) {
	later := time.Date(2026, 10, 26, 15, 4, 5, 0, time.Local)
	earlier := later.Add(-2 * time.Hour)
	var buf bytes.Buffer
	log := New(&buf, clockAt(later, earlier))
	log.Setup("linux", "amd64", "/bin/sh", "/work")

	lines := splitLines(t, buf.String())
	if len(lines) < 2 {
		t.Fatal(buf.String())
	}
	if stamp(lines[1]) != stamp(lines[0]) {
		t.Fatalf("timestamp moved backward:\n%s", buf.String())
	}
	if stamp(lines[0]) != "2026-10-26 15:04:05" {
		t.Fatalf("timestamp = %q", lines[0])
	}
}

func TestDuration_ThreeDecimalSeconds(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 15, 4, 5, 0, time.Local)
	stepDone := t0.Add(40 * time.Millisecond)
	runDone := t0.Add(65200 * time.Millisecond)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0, t0, t0, t0, t0, t0, stepDone, runDone))
	log.Setup("linux", "amd64", "/bin/sh", "/work")
	log.StepStarted("hello")
	code := 0
	log.StepFinished("hello", &code, nil)
	log.SummarySucceeded(1)

	out := buf.String()
	if !strings.Contains(out, "step hello finished duration 0.040s exit 0") {
		t.Fatalf("finish line missing:\n%s", out)
	}
	if !strings.Contains(out, "summary succeeded steps 1 duration 65.200s") {
		t.Fatalf("summary missing:\n%s", out)
	}
}

func TestTranscript_SucceededSteps(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 8, 1, 2, 0, time.Local)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0))
	log.Setup("linux", "amd64", "/bin/sh", "/work")
	code := 0
	log.StepStarted("hello")
	log.StepOutput("hello", []byte("Hel"))
	log.StepOutput("hello", []byte("lo from TRUSS\n"))
	log.StepFinished("hello", &code, nil)
	log.StepStarted("where")
	log.StepFinished("where", &code, nil)
	log.SummarySucceeded(2)

	got := occurrences(t, buf.String())
	want := []string{
		"setup environment ready",
		"setup os linux",
		"setup arch amd64",
		"setup shell /bin/sh",
		"setup directory /work",
		"step hello started",
		"step hello | Hello from TRUSS",
		"step hello finished duration 0.000s exit 0",
		"step where started",
		"step where finished duration 0.000s exit 0",
		"summary succeeded steps 2 duration 0.000s",
	}
	if len(got) != len(want) {
		t.Fatalf("occurrences = %q\n%s", got, buf.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("occurrence %d = %q, want %q\n%s", i, got[i], want[i], buf.String())
		}
	}
	where := strings.Index(buf.String(), "step where started")
	if strings.Contains(buf.String()[where:], " | ") {
		t.Fatalf("empty step wrote an output line:\n%s", buf.String())
	}
}

func TestTranscript_FailedStep(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 8, 1, 2, 0, time.Local)
	failAt := t0.Add(40 * time.Millisecond)
	runDone := t0.Add(1500 * time.Millisecond)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0, t0, t0, t0, t0, t0, failAt, runDone))
	log.Setup("linux", "amd64", "/bin/sh", "/work")
	log.StepStarted("second")
	code := 1
	log.StepFinished("second", &code, nil)
	log.SummaryFailed(2, "fail-fast failed at second (exit 1)")

	got := occurrences(t, buf.String())
	wantTail := []string{
		"step second finished duration 0.040s exit 1",
		"summary failed steps 2 duration 1.500s",
		"error fail-fast failed at second (exit 1)",
	}
	if len(got) < len(wantTail) {
		t.Fatalf("occurrences = %q", got)
	}
	tail := got[len(got)-len(wantTail):]
	if strings.Join(tail, "\n") != strings.Join(wantTail, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(tail, "\n"), strings.Join(wantTail, "\n"))
	}
}

func TestTranscript_StartFailure(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 8, 1, 2, 0, time.Local)
	failAt := t0.Add(40 * time.Millisecond)
	runDone := t0.Add(40 * time.Millisecond)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0, t0, t0, t0, t0, t0, failAt, runDone))
	log.Setup("linux", "amd64", "/bin/sh", "/work")
	log.StepStarted("missing")
	log.StepFinished("missing", nil, errString("start command: not found"))
	log.SummaryFailed(1, "demo failed at missing: start command: not found")

	got := occurrences(t, buf.String())
	var finish string
	for _, line := range got {
		if strings.HasPrefix(line, "step missing finished") {
			finish = line
		}
	}
	if finish != "step missing finished duration 0.040s error start command: not found" {
		t.Fatalf("finish = %q\n%s", finish, buf.String())
	}
	if strings.Contains(finish, "exit") {
		t.Fatalf("start failure includes an exit code: %s", finish)
	}
	wantTail := []string{
		"summary failed steps 1 duration 0.040s",
		"error demo failed at missing: start command: not found",
	}
	tail := got[len(got)-len(wantTail):]
	if strings.Join(tail, "\n") != strings.Join(wantTail, "\n") {
		t.Fatalf("got:\n%s", strings.Join(got, "\n"))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestStep_BlankAndPartialLines(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 8, 1, 2, 0, time.Local)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0))
	log.StepStarted("hello")
	log.StepOutput("hello", []byte("a\n\npartial"))
	code := 0
	log.StepFinished("hello", &code, nil)

	got := occurrences(t, buf.String())
	want := []string{
		"step hello started",
		"step hello | a",
		"step hello | ",
		"step hello | partial",
		"step hello finished duration 0.000s exit 0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func clockAt(times ...time.Time) func() time.Time {
	var i int
	return func() time.Time {
		if i >= len(times) {
			return times[len(times)-1]
		}
		t := times[i]
		i++
		return t
	}
}

func splitLines(t *testing.T, out string) []string {
	t.Helper()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("log does not end with a newline:\n%s", out)
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

var timestampLine = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) (.*)$`)

func stamp(line string) string {
	m := timestampLine.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

func occurrences(t *testing.T, out string) []string {
	t.Helper()
	lines := splitLines(t, out)
	got := make([]string, 0, len(lines))
	var prev string
	for _, line := range lines {
		m := timestampLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("line %q is missing a timestamp", line)
		}
		if prev != "" && m[1] < prev {
			t.Fatalf("timestamp %s is earlier than %s", m[1], prev)
		}
		prev = m[1]
		got = append(got, m[2])
	}
	return got
}
