package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Spec 028: --wait belongs to stop only, and a stopped daemon is fine.
func TestRobust_StopWait(t *testing.T) {
	s := newSession(t, false)
	if out := s.ok("daemon", "stop", "--wait"); !strings.Contains(out, "daemon is not running") {
		t.Fatalf("stop --wait without a daemon: %q", out)
	}
	if code, _, errOut := s.truss("daemon", "start", "--wait"); code != 1 || !isUsageError(errOut) {
		t.Fatalf("start --wait: %d %q", code, errOut)
	}
	s = newSession(t, true)
	if code, out, errOut := s.truss("daemon", "stop", "--wait"); code != 0 || !strings.Contains(out, "daemon stopped") {
		t.Fatalf("stop --wait: %d %q %q", code, out, errOut)
	}
}

func TestRobust_ADaemonLogOverTheLimitIsSetAside(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "daemon.log")
	if err := os.WriteFile(log, make([]byte, maxDaemonLog+1), 0o600); err != nil {
		t.Fatal(err)
	}
	rotateLog(log)
	if _, err := os.Stat(log + ".1"); err != nil {
		t.Fatalf("not set aside: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("daemon.log is still there: %v", err)
	}
	if err := os.WriteFile(log, []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	rotateLog(log)
	if _, err := os.Stat(log); err != nil {
		t.Fatalf("a small log was moved: %v", err)
	}
}

func TestRobust_KillTimeoutIsValidated(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f, kill_timeout: 5s}\nsteps:\n  - id: a\n    run: echo a\n    kill_timeout: soon\n")
	if code, _, errOut := s.truss("validate", "f.yaml"); code != 1 || !strings.Contains(errOut, "kill_timeout must be a duration") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

// --wait really lets an active run finish before the daemon stops.
func TestRobust_StopWaitLetsARunFinish(t *testing.T) {
	s := newSession(t, true)
	wait := "sleep 1"
	if runtime.GOOS == "windows" {
		wait = "ping -n 2 127.0.0.1 >nul"
	}
	s.write("slow.yaml", "version: \"1\"\nflow: {name: slow}\nsteps:\n  - id: a\n    run: "+wait+"\n")
	s.ok("register", "slow.yaml")
	s.ok("trigger", "slow")
	out := s.ok("daemon", "stop", "--wait")
	if !strings.Contains(out, "waiting for 1 run(s) to finish") || !strings.Contains(out, "daemon stopped") {
		t.Fatalf("stop --wait:\n%s", out)
	}
	runs, err := os.ReadDir(filepath.Join(s.home, "runs"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs: %v %v", runs, err)
	}
	data, err := os.ReadFile(filepath.Join(s.home, "runs", runs[0].Name(), "run.json"))
	if err != nil || !strings.Contains(string(data), `"status": "succeeded"`) {
		t.Fatalf("the run did not finish:\n%s %v", data, err)
	}
}
