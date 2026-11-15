package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/daemon"
	"github.com/afri-bit/truss/internal/home"
)

// session is a TRUSS home with a daemon serving it inside the test process.
type session struct {
	t   *testing.T
	env []string
	dir string
}

func newSession(t *testing.T, withDaemon bool) *session {
	t.Helper()
	h := home.Dir(t.TempDir())
	s := &session{t: t, env: append(os.Environ(), home.EnvVar+"="+string(h)), dir: t.TempDir()}
	if !withDaemon {
		return s
	}
	d, err := daemon.Start(context.Background(), daemon.Options{Home: h, Address: "127.0.0.1:0", Version: Version, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- d.Serve(context.Background()) }()
	t.Cleanup(func() {
		d.Stop()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	return s
}

// truss runs one CLI invocation in the session.
func (s *session) truss(args ...string) (int, string, string) {
	s.t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, s.dir, s.env, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// ok runs an invocation that must succeed and returns its stdout.
func (s *session) ok(args ...string) string {
	s.t.Helper()
	code, out, errOut := s.truss(args...)
	if code != 0 {
		s.t.Fatalf("truss %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}

func (s *session) write(name, body string) {
	s.t.Helper()
	if err := os.WriteFile(filepath.Join(s.dir, name), []byte(body), 0o600); err != nil {
		s.t.Fatal(err)
	}
}

var runID = regexp.MustCompile(`\d{8}-\d{6}-[0-9a-f]{6}`)

func TestCLI_RegisterTriggerLogsUnregister(t *testing.T) {
	s := newSession(t, true)
	s.write("truss.yaml", validPipeline(echoHello()))

	if out := s.ok("pipelines"); !strings.Contains(out, "no pipelines registered") {
		t.Fatalf("pipelines = %q", out)
	}
	if out := s.ok("register"); !strings.HasPrefix(out, "registered hello-world (") {
		t.Fatalf("register = %q", out)
	}
	if out := s.ok("pipelines"); !strings.Contains(out, "hello-world  ready") {
		t.Fatalf("pipelines = %q", out)
	}

	out := s.ok("trigger", "hello-world", "--follow")
	if !strings.Contains(out, "step hello | Hello from TRUSS") || !strings.Contains(out, "summary succeeded steps 1") {
		t.Fatalf("followed run:\n%s", out)
	}
	list := s.ok("runs", "hello-world")
	id := runID.FindString(list)
	if id == "" || !strings.Contains(list, "succeeded") || !strings.Contains(list, "manual") {
		t.Fatalf("runs = %q", list)
	}
	if again := s.ok("logs", id); again != out {
		t.Fatalf("logs differ from the followed run:\n%s", again)
	}
	if out := s.ok("cancel", id); !strings.Contains(out, "is succeeded") {
		t.Fatalf("cancel a finished run = %q", out)
	}

	s.ok("pause", "hello-world")
	if out := s.ok("pipelines"); !strings.Contains(out, "hello-world  paused") {
		t.Fatalf("pipelines = %q", out)
	}
	s.ok("resume", "hello-world")
	s.ok("unregister", "hello-world")
	if code, _, errOut := s.truss("trigger", "hello-world"); code != 1 || !strings.Contains(errOut, "not registered") {
		t.Fatalf("trigger after unregister: %d %q", code, errOut)
	}
}

func TestCLI_RegisterUnderAnotherName(t *testing.T) {
	s := newSession(t, true)
	s.write("ci.yaml", validPipeline(echoHello()))
	for _, args := range [][]string{{"register", "ci.yaml", "--name", "ci"}, {"register", "--name=ci2", "ci.yaml"}} {
		s.ok(args...)
	}
	if out := s.ok("pipelines"); !strings.Contains(out, "ci  ") || !strings.Contains(out, "ci2 ") {
		t.Fatalf("pipelines = %q", out)
	}
}

func TestCLI_FollowedFailedRunExitsOne(t *testing.T) {
	s := newSession(t, true)
	s.write("truss.yaml", validPipeline("exit 4"))
	s.ok("register")
	code, out, errOut := s.truss("trigger", "hello-world", "-f")
	if code != 1 || !strings.Contains(out, "summary failed") || !strings.Contains(errOut, "hello-world failed at hello (exit 4)") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
}

func TestCLI_RegisterRejectsAnInvalidDefinition(t *testing.T) {
	s := newSession(t, true)
	s.write("truss.yaml", "version: \"1\"\npipeline: {name: x}\nsteps: []\n")
	code, _, errOut := s.truss("register")
	if code != 1 || errOut != "error: pipeline must contain one or more steps\n" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestCLI_DaemonCommandsExplainAMissingDaemon(t *testing.T) {
	s := newSession(t, false)
	for _, args := range [][]string{{"pipelines"}, {"register"}, {"trigger", "x"}, {"runs"}, {"logs", "x"}, {"cancel", "x"}, {"desktop", "--no-open"}} {
		code, _, errOut := s.truss(args...)
		if code != 1 || !strings.Contains(errOut, "truss daemon start") {
			t.Errorf("truss %v: exit %d, stderr %q", args, code, errOut)
		}
	}
	if code, out, _ := s.truss("daemon", "status"); code != 1 || !strings.Contains(out, "not running") {
		t.Errorf("daemon status: %d %q", code, out)
	}
	if out := s.ok("daemon", "stop"); !strings.Contains(out, "not running") {
		t.Errorf("daemon stop = %q", out)
	}
}

func TestCLI_DaemonStatusAndDesktopAddress(t *testing.T) {
	s := newSession(t, true)
	if out := s.ok("daemon", "status"); !strings.Contains(out, "daemon is running") || !strings.Contains(out, "version    "+Version) {
		t.Fatalf("status = %q", out)
	}
	if out := s.ok("desktop", "--no-open"); !regexp.MustCompile(`^TRUSS Desktop: http://127\.0\.0\.1:\d+/#token=[0-9a-f]{48}\n$`).MatchString(out) {
		t.Fatalf("desktop = %q", out)
	}
	if out := s.ok("daemon", "start"); !strings.Contains(out, "already running") {
		t.Fatalf("start = %q", out)
	}
}

func TestCLI_BadArgumentsPrintUsage(t *testing.T) {
	s := newSession(t, false)
	cases := [][]string{
		{"daemon"}, {"daemon", "restart"}, {"daemon", "stop", "--address", "x"}, {"daemon", "start", "extra"},
		{"register", "a", "b"}, {"register", "--nope"}, {"register", "--name"},
		{"unregister"}, {"pipelines", "x"}, {"pause"}, {"resume", "a", "b"},
		{"trigger"}, {"trigger", "a", "--loud"}, {"runs", "a", "b"}, {"logs"}, {"cancel"},
		{"desktop", "x"}, {"validate", "a", "b"},
	}
	for _, args := range cases {
		code, out, errOut := s.truss(args...)
		if code != 1 || errOut != usageText || out != "" {
			t.Errorf("truss %v: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
	if code, _, errOut := s.truss("daemon", "start", "--max-runs", "0"); code != 1 || !strings.Contains(errOut, "--max-runs") {
		t.Errorf("max-runs 0: %d %q", code, errOut)
	}
}

func TestCLI_ValidateDescribesTheDefinition(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", `version: "1"
pipeline: {name: nightly, concurrency: 2}
on:
  schedule:
    - cron: "0 9 * * 1-5"
jobs:
  build:
    steps:
      - {id: compile, run: make}
  ship:
    needs: [build]
    steps:
      - {id: publish, run: make publish}
`)
	want := "truss.yaml is valid: pipeline nightly, 2 steps, concurrency 2\n" +
		"schedule 0 9 * * 1-5 (local time, missed: skip)\n" +
		"step build/compile\n" +
		"step ship/publish needs build/compile\n"
	if out := s.ok("validate"); out != want {
		t.Fatalf("validate =\n%s", out)
	}
	s.write("bad.yaml", "version: \"1\"\npipeline: {name: x}\non: {schedule: [{cron: \"99 * * * *\"}]}\nsteps: [{id: a, run: b}]\n")
	code, _, errOut := s.truss("validate", "bad.yaml")
	if code != 1 || !strings.Contains(errOut, "schedule 1: cron") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestUsage_NamesEveryCommand(t *testing.T) {
	for _, cmd := range []string{"run", "validate", "daemon start", "daemon stop", "daemon run", "desktop", "register", "unregister", "pipelines", "pause", "resume", "trigger", "runs", "logs", "cancel", "cache clear", "help", "version"} {
		if !strings.Contains(usageText, cmd) {
			t.Errorf("usage does not mention %q", cmd)
		}
	}
}
