package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const interruptDeadline = 10 * time.Second

// readPidWhenReady polls for a pid file written by the helper.
func readPidWhenReady(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(interruptDeadline)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pid file %s not written within %s", path, interruptDeadline)
	return 0
}

func requireGone(t *testing.T, what string, pid int) {
	t.Helper()
	deadline := time.Now().Add(interruptDeadline)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s (pid %d) is still alive", what, pid)
}

// yamlSingle quotes s as a YAML single-quoted scalar, so Windows paths and
// double quotes in a run string survive unchanged.
func yamlSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// writeBlockingPipeline writes truss.yaml with one Step that starts a
// grandchild and blocks, and an existing run log with known bytes.
func writeBlockingPipeline(t *testing.T, dir, pidfile string) (logPath string, previous []byte) {
	t.Helper()
	body := "version: \"1\"\npipeline:\n  name: interrupt\nsteps:\n  - id: wait\n    run: " +
		yamlSingle(helperCmd("spawn-grandchild", pidfile)) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "truss.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath = filepath.Join(dir, ".truss", "run.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	previous = []byte("previous log\nsecond line\n")
	if err := os.WriteFile(logPath, previous, 0o644); err != nil {
		t.Fatal(err)
	}
	return logPath, previous
}

// assertLogUntouched checks FR-017: the previous run log is byte-for-byte
// unchanged and no temporary log is left behind.
func assertLogUntouched(t *testing.T, logPath string, previous []byte) {
	t.Helper()
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	if !bytes.Equal(got, previous) {
		t.Fatalf("run log changed to %q, want %q", got, previous)
	}
	assertNoTemporaryLog(t, filepath.Dir(logPath))
}

func TestRun_CancelledContextEndsStepsAndKeepsLog(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	logPath, previous := writeBlockingPipeline(t, dir, pidfile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		code        int
		out, errOut string
	}
	done := make(chan outcome, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := Run(ctx, []string{"run"}, dir, os.Environ(), &stdout, &stderr)
		done <- outcome{code, stdout.String(), stderr.String()}
	}()

	grandchild := readPidWhenReady(t, pidfile)
	parent := readPidWhenReady(t, pidfile+".parent")
	cancel()

	var res outcome
	select {
	case res = <-done:
	case <-time.After(interruptDeadline):
		t.Fatal("Run did not return after the context was cancelled")
	}
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %s", res.code, res.errOut)
	}
	if !strings.Contains(res.errOut, "error: context canceled") {
		t.Fatalf("stderr = %q, want it to contain %q", res.errOut, "error: context canceled")
	}
	// The setup directory line prints the temp path, which contains this test's name.
	if all := strings.ToLower(withoutSetupDirectory(res.out) + res.errOut); strings.Contains(all, "cancelled") {
		t.Fatalf("output reports a cancelled status:\n%s\n%s", res.out, res.errOut)
	}
	requireGone(t, "grandchild", grandchild)
	requireGone(t, "step process", parent)
	assertLogUntouched(t, logPath, previous)
}

// withoutSetupDirectory drops the run log's "setup directory <path>" line so a
// temp path cannot match a word the tests look for in the status text.
func withoutSetupDirectory(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, " setup directory ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// cancelOnFinishedLine is a stdout that cancels the run's context the moment
// the run log reports a finished Step. The Step has then ended by itself, so
// it was never Stopped, and the context is done before the run is judged.
type cancelOnFinishedLine struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancelOnFinishedLine) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if strings.Contains(string(p), " finished duration ") {
		w.cancel()
	}
	return n, err
}

// TestRun_ContextCancelledAfterStepFinishedKeepsLogAndPrintsError covers
// FR-017: an interrupted run never saves a run log and never prints a normal
// failure, even when the Step exited on its own before the interrupt landed.
func TestRun_ContextCancelledAfterStepFinishedKeepsLogAndPrintsError(t *testing.T) {
	dir := t.TempDir()
	body := "version: \"1\"\npipeline:\n  name: interrupt\nsteps:\n  - id: bad\n    run: " +
		yamlSingle(helperCmd("exit", "3")) + "\n  - id: later\n    run: " +
		yamlSingle(helperCmd("exit", "0")) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "truss.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, ".truss", "run.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := []byte("previous log\nsecond line\n")
	if err := os.WriteFile(logPath, previous, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &cancelOnFinishedLine{cancel: cancel}
	var stderr bytes.Buffer

	code := Run(ctx, []string{"run"}, dir, os.Environ(), stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "error: context canceled") {
		t.Fatalf("stderr = %q, want it to contain %q (stdout:\n%s)", stderr.String(), "error: context canceled", stdout.String())
	}
	if all := strings.ToLower(withoutSetupDirectory(stdout.String())); strings.Contains(all, "failed") || strings.Contains(all, "cancelled") {
		t.Fatalf("output reports a final status:\n%s", stdout.String())
	}
	assertLogUntouched(t, logPath, previous)
}
