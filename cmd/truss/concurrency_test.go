package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Tests for spec 004 (step concurrency) through the CLI: the run log while
// Steps overlap, fail-fast stopping, limit and field validation, and the
// sequential baseline. The YAML field is pipeline.concurrency.

const runTimeout = 60 * time.Second

// invokeEnv runs the CLI with env, under a deadline so a missing feature
// fails the test instead of hanging it.
func invokeEnv(t *testing.T, cwd string, env []string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"run"}, cwd, env, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

type stepDef struct {
	id    string
	run   string
	needs []string
}

// definition renders truss.yaml. limit is a raw line under pipeline (for
// example "concurrency: 2") and is omitted when empty.
func definition(name, limit string, steps ...stepDef) string {
	var b strings.Builder
	b.WriteString("version: \"1\"\npipeline:\n  name: " + name + "\n")
	if limit != "" {
		b.WriteString("  " + limit + "\n")
	}
	b.WriteString("steps:\n")
	for _, s := range steps {
		b.WriteString("  - id: " + s.id + "\n")
		if len(s.needs) > 0 {
			b.WriteString("    needs: [" + strings.Join(s.needs, ", ") + "]\n")
		}
		b.WriteString("    run: " + yamlSingle(s.run) + "\n")
	}
	return b.String()
}

func writeDefinition(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "truss.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedLog(t *testing.T, dir string) (string, []byte) {
	t.Helper()
	logPath := filepath.Join(dir, ".truss", "run.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := []byte("previous log\nsecond line\n")
	if err := os.WriteFile(logPath, previous, 0o644); err != nil {
		t.Fatal(err)
	}
	return logPath, previous
}

// both joins helper commands with && so they run in order under sh and cmd.
func both(cmds ...string) string { return strings.Join(cmds, " && ") }

const stampLayout = "2006-01-02 15:04:05"

var stampPrefix = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) (.*)$`)

// splitLog returns the timestamps and the text after each timestamp.
func splitLog(t *testing.T, out string) ([]time.Time, []string) {
	t.Helper()
	var stamps []time.Time
	var texts []string
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		m := stampPrefix.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("line is not complete or has no timestamp: %q\n%s", line, out)
		}
		ts, err := time.ParseInLocation(stampLayout, m[1], time.Local)
		if err != nil {
			t.Fatal(err)
		}
		stamps = append(stamps, ts)
		texts = append(texts, m[2])
	}
	return stamps, texts
}

func indexOf(texts []string, prefix string) int {
	for i, s := range texts {
		if strings.HasPrefix(s, prefix) {
			return i
		}
	}
	return -1
}

var (
	outputLine = regexp.MustCompile(`^step ([a-z]+) \| (.*)$`)
	durationRe = regexp.MustCompile(`duration \d+\.\d{3}s`)
	setupRe    = regexp.MustCompile(`^(setup (?:os|arch|shell|directory)) .*$`)
)

// normalized drops timestamps and variable values, leaving the log shape.
func normalized(t *testing.T, out string) []string {
	t.Helper()
	_, texts := splitLog(t, out)
	for i, s := range texts {
		s = durationRe.ReplaceAllString(s, "duration D")
		texts[i] = setupRe.ReplaceAllString(s, "$1 X")
	}
	return texts
}

// --- US2: one readable log while Steps overlap (T018, T019) ---

func TestRun_ConcurrentLogLinesAreCompleteLabelledAndOrdered(t *testing.T) {
	dir := t.TempDir()
	am, bm := filepath.Join(dir, "a.mark"), filepath.Join(dir, "b.mark")
	writeDefinition(t, dir, definition("overlap", "concurrency: 2",
		stepDef{id: "a", run: both(helperCmd("touch-and-wait", am, bm), helperCmd("print-lines", "20", "A"))},
		stepDef{id: "b", run: both(helperCmd("touch-and-wait", bm, am), helperCmd("print-lines", "20", "B"))},
	))
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	stamps, texts := splitLog(t, out)
	for i := 1; i < len(stamps); i++ {
		if stamps[i].Before(stamps[i-1]) {
			t.Fatalf("timestamp went backwards at line %d:\n%s", i+1, out)
		}
	}
	next := map[string]int{"a": 1, "b": 1}
	for _, s := range texts {
		m := outputLine.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		id, body := m[1], m[2]
		want := strings.ToUpper(id) + " " + strconv.Itoa(next[id])
		if body != want {
			t.Fatalf("step %s line = %q, want %q (mixed, split or out of order)\n%s", id, body, want, out)
		}
		next[id]++
	}
	if next["a"] != 21 || next["b"] != 21 {
		t.Fatalf("lines seen a=%d b=%d, want 20 each\n%s", next["a"]-1, next["b"]-1, out)
	}
	a, b := indexOf(texts, "step a started"), indexOf(texts, "step b started")
	if a < 0 || b < a {
		t.Fatalf("started Steps not listed in start order a, b:\n%s", out)
	}
	if got := readRunLog(t, dir); got != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", got, out)
	}
}

func TestRun_ConcurrentLogPartialLineAppearsOnceComplete(t *testing.T) {
	dir := t.TempDir()
	am, bm := filepath.Join(dir, "a.mark"), filepath.Join(dir, "b.mark")
	writeDefinition(t, dir, definition("partial", "concurrency: 2",
		stepDef{id: "a", run: both(helperCmd("touch-and-wait", am, bm), helperCmd("partial", "tail-text"))},
		stepDef{id: "b", run: both(helperCmd("touch-and-wait", bm, am), helperCmd("print-lines", "10", "B"))},
	))
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	_, texts := splitLog(t, out)
	count := 0
	for i, s := range texts {
		if strings.Contains(s, "tail-text") {
			count++
			if s != "step a | tail-text" {
				t.Fatalf("partial line = %q, want %q", s, "step a | tail-text")
			}
			fin := indexOf(texts, "step a finished")
			if fin != i+1 {
				t.Fatalf("partial line is not written at the end of step a:\n%s", out)
			}
		}
	}
	if count != 1 {
		t.Fatalf("partial text appears %d times, want 1:\n%s", count, out)
	}
	if got := readRunLog(t, dir); got != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", got, out)
	}
}

func TestRun_ConcurrentLogKeepsSetupAndSummaryShape(t *testing.T) {
	dir := t.TempDir()
	am, bm := filepath.Join(dir, "a.mark"), filepath.Join(dir, "b.mark")
	writeDefinition(t, dir, definition("shape", "concurrency: 3",
		stepDef{id: "build", run: both(helperCmd("touch-and-wait", am, bm), helperCmd("print-lines", "2", "X"))},
		stepDef{id: "lint", run: both(helperCmd("touch-and-wait", bm, am), helperCmd("print-lines", "2", "Y"))},
		stepDef{id: "test", needs: []string{"build"}, run: helperCmd("print-lines", "1", "Z")},
	))
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	_, texts := splitLog(t, out)
	want := []string{"setup environment ready", "setup os ", "setup arch ", "setup shell ", "setup directory "}
	for i, prefix := range want {
		if i >= len(texts) || !strings.HasPrefix(texts[i], prefix) {
			t.Fatalf("setup section line %d should start %q:\n%s", i+1, prefix, out)
		}
	}
	last := texts[len(texts)-1]
	if !regexp.MustCompile(`^summary succeeded steps 3 duration \d+\.\d{3}s$`).MatchString(last) {
		t.Fatalf("summary = %q", last)
	}
	b, l, tt := indexOf(texts, "step build started"), indexOf(texts, "step lint started"), indexOf(texts, "step test started")
	if b < 0 || l < b || tt < l {
		t.Fatalf("Steps not listed in start order build, lint, test:\n%s", out)
	}
	for _, id := range []string{"build", "lint", "test"} {
		if indexOf(texts, "step "+id+" finished duration ") < 0 {
			t.Fatalf("step %s has no finish line:\n%s", id, out)
		}
	}
}

func TestRun_ConcurrentLogHidesLimitDependenciesAndEnvironment(t *testing.T) {
	dir := t.TempDir()
	const name, value = "TRUSS_TEST_FAKE_SECRET_NAME", "fake-secret-value-7391"
	writeDefinition(t, dir, definition("hidden", "concurrency: 3",
		stepDef{id: "build", run: helperCmd("print-lines", "1", "X")},
		stepDef{id: "lint", run: helperCmd("print-lines", "1", "Y")},
		stepDef{id: "test", needs: []string{"build"}, run: helperCmd("print-lines", "1", "Z")},
	))
	env := append(os.Environ(), name+"="+value)
	code, out, errOut := invokeEnv(t, dir, env)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	logged := withoutSetupDirectory(out)
	saved := withoutSetupDirectory(readRunLog(t, dir))
	for _, text := range []string{logged, saved, errOut} {
		for _, banned := range []string{"concurrency", "needs", "depend", name, value} {
			if strings.Contains(strings.ToLower(text), strings.ToLower(banned)) {
				t.Fatalf("log shows %q:\n%s", banned, text)
			}
		}
	}
}

// --- US3: a failure stops the whole run (T022) ---

func TestRun_ConcurrentFailureStopsRunningStepsAndOmitsReadyOnes(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	am := filepath.Join(dir, "a.mark")
	bDone := filepath.Join(dir, "b.done")
	cDone := filepath.Join(dir, "c.done")
	// a fails only after b's grandchild is running, so b is mid-flight.
	writeDefinition(t, dir, definition("stopper", "concurrency: 2",
		stepDef{id: "a", run: helperCmd("touch-and-wait", am, pidfile, "1")},
		stepDef{id: "b", run: both(helperCmd("spawn-grandchild", pidfile), helperCmd("touch", bDone))},
		stepDef{id: "c", run: helperCmd("touch", cDone)},
	))
	logPath, _ := seedLog(t, dir)
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	if strings.Contains(errOut, "failed at") {
		t.Fatalf("stderr duplicates the run log: %s", errOut)
	}
	_, texts := splitLog(t, out)
	if i := indexOf(texts, "step c"); i >= 0 {
		t.Fatalf("step c is in the log (it never started):\n%s", out)
	}
	if indexOf(texts, "step a started") < 0 || indexOf(texts, "step b started") < indexOf(texts, "step a started") {
		t.Fatalf("a and b did not start in order:\n%s", out)
	}
	if i := indexOf(texts, "step a finished"); i < 0 || !strings.HasSuffix(texts[i], " exit 1") {
		t.Fatalf("step a should finish with exit 1:\n%s", out)
	}
	stop := indexOf(texts, "step b stopped because step a failed")
	if stop < 0 {
		t.Fatalf("missing %q:\n%s", "step b stopped because step a failed", out)
	}
	if stop+1 >= len(texts) || !strings.HasPrefix(texts[stop+1], "step b finished duration ") {
		t.Fatalf("the stopped line is not immediately before b's finish line:\n%s", out)
	}
	fin := texts[stop+1]
	hasExit := regexp.MustCompile(` exit \d+$`).MatchString(fin)
	if !hasExit && !strings.Contains(fin, " error ") {
		t.Fatalf("b's finish line has no Exit Code or diagnostic: %q", fin)
	}
	if strings.Contains(fin, "exit -1") {
		t.Fatalf("b's finish line shows a made-up exit code: %q", fin)
	}
	if indexOf(texts, "summary failed steps 2 ") < 0 {
		t.Fatalf("summary should report failed with 2 started Steps:\n%s", out)
	}
	if !strings.Contains(out, "error stopper failed at a (exit 1)") {
		t.Fatalf("error line should name the pipeline and step a:\n%s", out)
	}
	if got := readRunLog(t, dir); got != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", got, out)
	}
	if got, _ := os.ReadFile(logPath); string(got) == "previous log\nsecond line\n" {
		t.Fatalf("the failed run did not save its log")
	}
	for _, marker := range []string{bDone, cDone} {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("a stopped or unstarted step wrote %s: %v", marker, err)
		}
	}
	requireGone(t, "grandchild of b", readPidWhenReady(t, pidfile))
	requireGone(t, "shell of b", readPidWhenReady(t, pidfile+".parent"))
}

func TestRun_StepKilledBySignalLogsDiagnosticWithoutExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no signal death; a killed process reports an Exit Code there")
	}
	dir := t.TempDir()
	// The shell kills itself, so the host reports no Exit Code for the Step.
	writeDefinition(t, dir, definition("signalled", "", stepDef{id: "k", run: "kill -KILL $$"}))
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
	}
	_, texts := splitLog(t, out)
	i := indexOf(texts, "step k finished duration ")
	if i < 0 {
		t.Fatalf("no finish line:\n%s", out)
	}
	fin := texts[i]
	if !regexp.MustCompile(` error \S`).MatchString(fin) || strings.Contains(fin, " exit ") {
		t.Fatalf("finish line = %q, want an error diagnostic and no exit field", fin)
	}
	if strings.Contains(out, "exit -1") {
		t.Fatalf("log shows exit -1:\n%s", out)
	}
}

// --- US4: invalid limits and unknown fields (T027) ---

func TestRun_RejectsInvalidConcurrencyLimit(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		shows string
	}{
		{"zero", "concurrency: 0", `"0"`},
		{"negative", "concurrency: -1", `"-1"`},
		{"fraction", "concurrency: 1.5", `"1.5"`},
		{"text", `concurrency: "two"`, "two"},
		{"boolean", "concurrency: true", `"true"`},
		{"list", "concurrency: []", "a list"},
		{"empty", "concurrency:", ""},
		{"empty text", `concurrency: ""`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "ran.txt")
			writeDefinition(t, dir, definition("bad-limit", tt.line,
				stepDef{id: "a", run: helperCmd("touch", marker)}))
			logPath, previous := seedLog(t, dir)
			code, out, errOut := invokeEnv(t, dir, os.Environ())
			if code != 1 {
				t.Fatalf("exit = %d, stderr = %s", code, errOut)
			}
			if !strings.HasPrefix(errOut, "error: ") || !strings.Contains(errOut, "concurrency") {
				t.Fatalf("stderr = %q, want an error naming concurrency", errOut)
			}
			if !strings.Contains(errOut, "whole number of at least 1") || !strings.Contains(errOut, tt.shows) {
				t.Fatalf("stderr = %q, want the rule and the offending value %q", errOut, tt.shows)
			}
			if out != "" {
				t.Fatalf("stdout = %q", out)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("a step command ran: %v", err)
			}
			assertLogUntouched(t, logPath, previous)
		})
	}
}

func TestRun_RejectsMisspelledConcurrencyField(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran.txt")
	writeDefinition(t, dir, definition("typo", "concurency: 2",
		stepDef{id: "a", run: helperCmd("touch", marker)}))
	logPath, previous := seedLog(t, dir)
	code, out, errOut := invokeEnv(t, dir, os.Environ())
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.HasPrefix(errOut, "error: ") || !strings.Contains(errOut, `unknown field "concurency" in pipeline`) {
		t.Fatalf("stderr = %q", errOut)
	}
	if out != "" {
		t.Fatalf("stdout = %q", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("a step command ran: %v", err)
	}
	assertLogUntouched(t, logPath, previous)
}

// --- US5: no limit and limit 1 keep the spec 002/003 behavior (T031) ---

func TestRun_NoLimitAndLimitOneKeepSequentialLogShape(t *testing.T) {
	for _, limit := range []string{"", "concurrency: 1"} {
		name := "no limit"
		if limit != "" {
			name = limit
		}
		t.Run("success/"+name, func(t *testing.T) {
			dir := t.TempDir()
			writeDefinition(t, dir, definition("seq-ok", limit,
				stepDef{id: "first", run: helperCmd("print-lines", "2", "one")},
				stepDef{id: "second", needs: []string{"first"}, run: helperCmd("print-lines", "1", "two")},
			))
			code, out, errOut := invokeEnv(t, dir, os.Environ())
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
			}
			want := []string{
				"setup environment ready", "setup os X", "setup arch X", "setup shell X", "setup directory X",
				"step first started", "step first | one 1", "step first | one 2", "step first finished duration D exit 0",
				"step second started", "step second | two 1", "step second finished duration D exit 0",
				"summary succeeded steps 2 duration D",
			}
			assertShape(t, normalized(t, out), want)
			if got := readRunLog(t, dir); got != out {
				t.Fatalf("saved log =\n%s\nstdout =\n%s", got, out)
			}
		})
		t.Run("failure/"+name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "third.txt")
			writeDefinition(t, dir, definition("fail-fast", limit,
				stepDef{id: "first", run: helperCmd("print-lines", "1", "one")},
				stepDef{id: "second", run: helperCmd("exit", "1")},
				stepDef{id: "third", run: helperCmd("touch", marker)},
			))
			code, out, errOut := invokeEnv(t, dir, os.Environ())
			if code != 1 {
				t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
			}
			want := []string{
				"setup environment ready", "setup os X", "setup arch X", "setup shell X", "setup directory X",
				"step first started", "step first | one 1", "step first finished duration D exit 0",
				"step second started", "step second finished duration D exit 1",
				"summary failed steps 2 duration D",
				"error fail-fast failed at second (exit 1)",
			}
			assertShape(t, normalized(t, out), want)
			if got := readRunLog(t, dir); got != out {
				t.Fatalf("saved log =\n%s\nstdout =\n%s", got, out)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("third step ran: %v", err)
			}
		})
	}
}

func assertShape(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("log shape =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// FR-014: when a Step's shell exits, processes the Step left behind end too.
func TestRun_StepEndsItsBackgroundProcessesWhenItFinishes(t *testing.T) {
	for _, limit := range []string{"", "concurrency: 2"} {
		t.Run("limit "+limit, func(t *testing.T) {
			dir := t.TempDir()
			pidfile := filepath.Join(dir, "child.pid")
			writeDefinition(t, dir, definition("leftover", limit,
				stepDef{id: "bg", run: helperCmd("spawn-grandchild", pidfile, "leave")}))
			code, out, errOut := invokeEnv(t, dir, os.Environ())
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
			}
			pid := readPidWhenReady(t, pidfile)
			if pidAlive(pid) {
				t.Fatalf("background process %d outlived its Step", pid)
			}
		})
	}
}

// FR-015: Steps run in the invoker's directory with the invoker's environment,
// and TRUSS runs nothing but the Step run strings.
func TestRun_StepsKeepInvokerDirectoryEnvironmentAndRunNothingElse(t *testing.T) {
	for _, limit := range []string{"", "concurrency: 2"} {
		t.Run("limit "+limit, func(t *testing.T) {
			dir := t.TempDir()
			const name, value = "TRUSS_TEST_PASSTHROUGH", "passthrough-value-4242"
			marker := filepath.Join(dir, "only.txt")
			writeDefinition(t, dir, definition("invoker", limit,
				stepDef{id: "where", run: helperCmd("print-cwd")},
				stepDef{id: "env", run: helperCmd("print-env", name)},
				stepDef{id: "mark", run: helperCmd("touch", marker)},
			))
			code, out, errOut := invokeEnv(t, dir, append(os.Environ(), name+"="+value))
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s\n%s", code, errOut, out)
			}
			_, texts := splitLog(t, out)
			var cwd, envOut string
			for _, s := range texts {
				if m := outputLine.FindStringSubmatch(s); m != nil {
					switch m[1] {
					case "where":
						cwd = m[2]
					case "env":
						envOut = m[2]
					}
				}
			}
			wantDir, _ := filepath.EvalSymlinks(dir)
			gotDir, _ := filepath.EvalSymlinks(cwd)
			if gotDir == "" || gotDir != wantDir {
				t.Fatalf("step ran in %q, want %q", cwd, dir)
			}
			if envOut != value {
				t.Fatalf("step saw env value %q, want %q", envOut, value)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				switch e.Name() {
				case "truss.yaml", ".truss", "only.txt":
				default:
					t.Fatalf("unexpected file %q: a command other than a Step ran", e.Name())
				}
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatalf("marker step did not run: %v", err)
			}
		})
	}
}
