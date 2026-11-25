package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

// Tests for spec 010: `truss run --track` against a daemon that runs inside
// the test process.

func TestRunTrack_ReportsRunStepsAndLogToTheDaemon(t *testing.T) {
	s := newSession(t, true)
	s.write("pipeline.yaml", definition("tracked", "",
		stepDef{id: "one", run: helperCmd("print-lines", "2", "first")},
		stepDef{id: "two", run: helperCmd("print-lines", "1", "second"), needs: []string{"one"}},
	))
	code, out, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	id := runID.FindString(errOut)
	if id == "" || !strings.Contains(errOut, "tracking as run "+id) {
		t.Fatalf("stderr does not name the run: %q", errOut)
	}
	// The run log on stdout is unchanged by --track.
	if !strings.Contains(out, "step one | first 2") || !strings.Contains(out, "summary succeeded steps 2") || strings.Contains(out, "tracking") {
		t.Fatalf("stdout:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "state", "pipeline", "run.log")); err != nil {
		t.Fatalf("the local run log is gone: %v", err)
	}

	list := s.ok("runs", "tracked")
	if !strings.Contains(list, id) || !strings.Contains(list, "succeeded") || !strings.Contains(list, "local") {
		t.Fatalf("runs = %q", list)
	}
	if logs := s.ok("logs", id); logs != out {
		t.Fatalf("the daemon's log differs from the local one:\n%s\n--- local:\n%s", logs, out)
	}
	run := getRun(t, s, id)
	if run.Trigger != "local" || run.Status != "succeeded" || len(run.Steps) != 2 {
		t.Fatalf("run = %+v", run)
	}
	for _, step := range run.Steps {
		if step.Status != "succeeded" || step.ExitCode == nil || *step.ExitCode != 0 || step.StartedAt == nil || step.FinishedAt == nil {
			t.Fatalf("step = %+v", step)
		}
	}
	if len(run.Steps[1].Needs) != 1 {
		t.Fatalf("step graph lost: %+v", run.Steps)
	}
}

func TestRunTrack_FailureKeepsTheExitCodeAndRecordsTheFailure(t *testing.T) {
	s := newSession(t, true)
	s.write("pipeline.yaml", definition("broken", "",
		stepDef{id: "boom", run: helperCmd("exit", "3")},
		stepDef{id: "after", run: helperCmd("exit", "0"), needs: []string{"boom"}},
	))
	code, _, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 1 {
		t.Fatalf("exit = %d, stderr %s", code, errOut)
	}
	run := getRun(t, s, runID.FindString(errOut))
	if run.Status != "failed" || run.Reason != "broken failed at boom (exit 3)" {
		t.Fatalf("run = %+v", run)
	}
	if run.Steps[0].Status != "failed" || *run.Steps[0].ExitCode != 3 || run.Steps[1].Status != "skipped" {
		t.Fatalf("steps = %+v", run.Steps)
	}
}

func TestRunTrack_UsesTheCallersEnvironmentNotTheDaemons(t *testing.T) {
	s := newSession(t, true)
	s.env = append(s.env, "TRUSS_TRACK_PROBE=from-the-caller")
	s.write("pipeline.yaml", definition("hello-world", "", stepDef{id: "hello", run: helperCmd("print-env", "TRUSS_TRACK_PROBE")}))
	code, out, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 0 || !strings.Contains(out, "from-the-caller") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
}

func TestRunTrack_WithoutADaemonFailsBeforeRunning(t *testing.T) {
	s := newSession(t, false)
	marker := filepath.Join(s.dir, "ran")
	s.write("pipeline.yaml", definition("hello-world", "", stepDef{id: "hello", run: helperCmd("touch", marker)}))
	code, out, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 1 || out != "" || !strings.Contains(errOut, "truss daemon start") || !strings.Contains(errOut, "--track") {
		t.Fatalf("exit %d\nstdout: %q\nstderr: %q", code, out, errOut)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the pipeline ran although tracking was impossible: %v", err)
	}
	// Without --track nothing changes: no daemon is needed.
	if code, _, errOut := s.truss("run", "pipeline.yaml"); code != 0 {
		t.Fatalf("plain run: %d %q", code, errOut)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("plain run did not run: %v", err)
	}
}

func TestRunTrack_InvalidDefinitionNeverReachesTheDaemon(t *testing.T) {
	s := newSession(t, true)
	s.write("pipeline.yaml", "version: \"1\"\npipeline: {name: x}\nsteps: []\n")
	code, _, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 1 || !strings.Contains(errOut, "a flow must contain one or more steps") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out := s.ok("runs"); !strings.Contains(out, "no runs yet") {
		t.Fatalf("runs = %q", out)
	}
}

func TestRunTrack_TooManyArgumentsPrintUsage(t *testing.T) {
	s := newSession(t, false)
	if code, _, errOut := s.truss("run", "a", "b", "--track"); code != 1 || errOut != usageText {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestRunTrack_DaemonCancelStopsTheLocalRun(t *testing.T) {
	s := newSession(t, true)
	pidfile := filepath.Join(s.dir, "pid")
	s.write("pipeline.yaml", definition("stuck", "", stepDef{id: "block", run: helperCmd("block", pidfile)}))
	res := s.start("run", "pipeline.yaml", "--track")
	if !pollFile(pidfile, 30*time.Second) {
		t.Fatal("the step never started")
	}
	id := runID.FindString(s.ok("runs", "stuck"))
	if id == "" || !strings.Contains(s.ok("runs", "stuck"), "running") {
		t.Fatal("the tracked run is not listed as running")
	}
	s.ok("cancel", id)
	r := res.wait(t, 30*time.Second)
	if r.code != 1 || !strings.Contains(r.stderr, "cancelled from the daemon") {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	run := getRun(t, s, id)
	if run.Status != "cancelled" || !strings.Contains(run.Reason, "cancelled from the daemon") {
		t.Fatalf("run = %+v", run)
	}
	// An interrupted local run saves no run log (spec 001 FR-017); this one is no exception.
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "state", "pipeline", "run.log")); !os.IsNotExist(err) {
		t.Fatalf("run log exists: %v", err)
	}
}

func TestRunTrack_RunSurvivesTheDaemonStoppingMidRun(t *testing.T) {
	s := newSession(t, true)
	mine, other := filepath.Join(s.dir, "started"), filepath.Join(s.dir, "release")
	s.write("pipeline.yaml", definition("outlive", "", stepDef{id: "wait", run: helperCmd("touch-and-wait", mine, other)}))
	res := s.start("run", "pipeline.yaml", "--track")
	if !pollFile(mine, 30*time.Second) {
		t.Fatal("the step never started")
	}
	s.ok("daemon", "stop")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := res.wait(t, 30*time.Second)
	if r.code != 0 || !strings.Contains(r.stdout, "summary succeeded steps 1") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "warning:") {
		t.Fatalf("no warning that tracking stopped: %q", r.stderr)
	}
}

type result struct {
	code           int
	stdout, stderr string
}

type pending struct{ ch chan result }

// start runs one CLI invocation in the background.
func (s *session) start(args ...string) *pending {
	p := &pending{ch: make(chan result, 1)}
	go func() {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, s.dir, s.env, &stdout, &stderr)
		p.ch <- result{code, stdout.String(), stderr.String()}
	}()
	return p
}

func (p *pending) wait(t *testing.T, d time.Duration) result {
	t.Helper()
	select {
	case r := <-p.ch:
		return r
	case <-time.After(d):
		t.Fatal("the command did not finish")
		return result{}
	}
}

// getRun reads one run from the session's daemon.
func getRun(t *testing.T, s *session, id string) api.Run {
	t.Helper()
	h, err := home.Resolve(s.env)
	if err != nil {
		t.Fatal(err)
	}
	c, err := api.Dial(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	run, err := c.Run(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRunTrack_InterruptClosesTheRunAsCancelled(t *testing.T) {
	s := newSession(t, true)
	pidfile := filepath.Join(s.dir, "pid")
	s.write("pipeline.yaml", definition("stuck", "", stepDef{id: "block", run: helperCmd("block", pidfile)}))
	ctx, interrupt := context.WithCancel(context.Background())
	defer interrupt()
	done := make(chan result, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := Run(ctx, []string{"run", "pipeline.yaml", "--track"}, s.dir, s.env, &stdout, &stderr)
		done <- result{code, stdout.String(), stderr.String()}
	}()
	if !pollFile(pidfile, 30*time.Second) {
		t.Fatal("the step never started")
	}
	id := runID.FindString(s.ok("runs", "stuck"))
	if id == "" {
		t.Fatal("the tracked run is not listed")
	}
	interrupt() // Ctrl-C
	var r result
	select {
	case r = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the run did not end after the interrupt")
	}
	if r.code != 1 || strings.Contains(r.stderr, "cancelled from the daemon") {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	run := getRun(t, s, id)
	if run.Status != "cancelled" || run.Reason != "interrupted" || run.Steps[0].Status != "cancelled" {
		t.Fatalf("run = %+v", run)
	}
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "state", "pipeline", "run.log")); !os.IsNotExist(err) {
		t.Fatalf("run log exists: %v", err)
	}
}

func TestRunTrack_TheDaemonLogIsRedactedToo(t *testing.T) {
	s := newSession(t, true)
	s.setSecret("TRUSS_TEST_TOKEN", canary)
	s.write("pipeline.yaml", secretPipelineText())
	code, out, errOut := s.truss("run", "pipeline.yaml", "--track")
	if code != 0 || strings.Contains(out, canary) {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	id := runID.FindString(errOut)
	logs := s.ok("logs", id)
	if strings.Contains(logs, canary) || !strings.Contains(logs, "***") {
		t.Fatalf("the tracked log leaks the secret or lacks the mask:\n%s", logs)
	}
	run := getRun(t, s, id)
	if strings.Contains(run.Reason, canary) {
		t.Fatalf("reason leaks: %q", run.Reason)
	}
}
