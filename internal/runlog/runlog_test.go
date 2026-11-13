package runlog

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

// forwardClock is safe for concurrent use and only moves forward.
func forwardClock(start time.Time) func() time.Time {
	var mu sync.Mutex
	cur := start
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		cur = cur.Add(250 * time.Millisecond)
		return cur
	}
}

func TestLog_ConcurrentOutput(t *testing.T) {
	const workers = 8
	const lines = 20
	var buf bytes.Buffer
	log := New(&buf, forwardClock(time.Date(2026, 10, 26, 8, 0, 0, 0, time.Local)))

	var wg sync.WaitGroup
	for w := range workers {
		id := fmt.Sprintf("s%d", w)
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.StepStarted(id)
			var text strings.Builder
			for n := range lines {
				fmt.Fprintf(&text, "%s line %d\n", id, n)
			}
			data := []byte(text.String())
			// Split at arbitrary points, including inside a line.
			for i := 0; i < len(data); {
				size := 1 + (i*7+w)%11
				end := min(i+size, len(data))
				log.StepOutput(id, data[i:end])
				i = end
			}
			code := 0
			log.StepFinished(id, &code, nil)
		}()
	}
	wg.Wait()

	out := buf.String()
	got := splitLines(t, out)
	outLine := regexp.MustCompile(`^\S+ \S+ step (s\d) \| (s\d) line (\d+)$`)
	ctrlLine := regexp.MustCompile(`^\S+ \S+ step (s\d) (started|finished duration \d+\.\d{3}s exit 0)$`)
	next := map[string]int{}
	var prev string
	for _, line := range got {
		if strings.Count(line, " step ") != 1 {
			t.Fatalf("line must carry exactly one step label: %q\n%s", line, out)
		}
		ts := stamp(line)
		if ts == "" || (prev != "" && ts < prev) {
			t.Fatalf("bad or decreasing timestamp in %q (previous %q)", line, prev)
		}
		prev = ts
		if m := outLine.FindStringSubmatch(line); m != nil {
			if m[1] != m[2] {
				t.Fatalf("output of %s labelled %s: %q", m[2], m[1], line)
			}
			n, _ := strconv.Atoi(m[3])
			if n != next[m[1]] {
				t.Fatalf("step %s line %d out of order, want %d\n%s", m[1], n, next[m[1]], out)
			}
			next[m[1]]++
			continue
		}
		if !ctrlLine.MatchString(line) {
			t.Fatalf("incomplete or malformed line: %q\n%s", line, out)
		}
	}
	for w := range workers {
		if id := fmt.Sprintf("s%d", w); next[id] != lines {
			t.Fatalf("step %s wrote %d output lines, want %d", id, next[id], lines)
		}
	}
}

func TestLog_StepStopped(t *testing.T) {
	t0 := time.Date(2026, 10, 26, 8, 1, 2, 0, time.Local)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0))
	log.StepStarted("b")
	log.StepOutput("b", []byte("half a li"))
	log.StepStopped("b", "a")
	code := 1
	log.StepFinished("b", &code, nil)

	got := occurrences(t, buf.String())
	want := []string{
		"step b started",
		"step b | half a li",
		"step b stopped because step a failed",
		"step b finished duration 0.000s exit 1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTranscript_RetryContinueAndSkippedLines(t *testing.T) {
	t0 := time.Date(2026, 11, 12, 8, 0, 0, 0, time.Local)
	var buf bytes.Buffer
	log := New(&buf, clockAt(t0))
	log.Setup("linux", "amd64", "/bin/sh", "/work")
	one, zero := 1, 0
	log.StepStarted("flaky")
	log.StepFinished("flaky", &one, nil)
	log.StepRetrying("flaky", 2, 3, 1500*time.Millisecond)
	log.StepStarted("flaky")
	log.StepFinished("flaky", &zero, nil)
	log.StepStarted("lint")
	log.StepFinished("lint", &one, nil)
	log.StepContinued("lint")
	log.StepSkipped("report", "step lint did not succeed")
	log.StepSkipped("deploy", "its if condition is false")
	log.SummarySucceeded(2)

	got := occurrences(t, buf.String())[5:]
	want := []string{
		"step flaky started",
		"step flaky finished duration 0.000s exit 1",
		"step flaky retry 2 of 3 in 1.500s",
		"step flaky started",
		"step flaky finished duration 0.000s exit 0",
		"step lint started",
		"step lint finished duration 0.000s exit 1",
		"step lint failed but the run continues (continue-on-error)",
		"step report skipped because step lint did not succeed",
		"step deploy skipped because its if condition is false",
		"summary succeeded steps 2 duration 0.000s",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTranscript_StepsWithoutRetriesKeepTheirLines(t *testing.T) {
	// The first attempt of a step looks exactly like a step that has no
	// retries: only the extra lines are new.
	var buf bytes.Buffer
	log := New(&buf, clockAt(time.Date(2026, 11, 12, 8, 0, 0, 0, time.Local)))
	one := 1
	log.StepStarted("a")
	log.StepFinished("a", &one, nil)
	got := occurrences(t, buf.String())
	if len(got) != 2 || got[0] != "step a started" || got[1] != "step a finished duration 0.000s exit 1" {
		t.Fatalf("lines = %q", got)
	}
}
