package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Spec 030: truss run --notify sends the file's notifications, also with
// --env (review: a sliced flow lost its notify block), and a failing
// notification does not change the exit code.
func TestNotify_LocalRunNotifies(t *testing.T) {
	s := newSession(t, false)
	out := filepath.Join(s.dir, "told.txt")
	tell := "echo $TRUSS_NOTIFY_STATUS >> \"" + out + "\""
	if runtime.GOOS == "windows" {
		tell = "echo %TRUSS_NOTIFY_STATUS%>> \"" + out + "\""
	}
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nnotify:\n  on: always\n  run: '"+strings.ReplaceAll(tell, "'", "''")+"'\nsteps:\n  - id: a\n    run: exit 3\n")
	if code, _, _ := s.truss("run", "f.yaml", "--notify", "--env", "X=1"); code != 1 {
		t.Fatalf("exit %d, want 1 (the run failed)", code)
	}
	data, err := os.ReadFile(out)
	if err != nil || strings.TrimSpace(string(data)) != "failed" {
		t.Fatalf("told = %q, %v", data, err)
	}
	s.write("g.yaml", "version: \"1\"\nflow: {name: g}\nnotify: {on: always, run: exit 7}\nsteps:\n  - id: a\n    run: echo ok\n")
	code, _, errOut := s.truss("run", "g.yaml", "--notify")
	if code != 0 || !strings.Contains(errOut, "note: notification not sent: run: exit 7") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	s.write("h.yaml", "version: \"1\"\nflow: {name: h}\nsteps:\n  - id: a\n    run: echo ok\n")
	if _, _, errOut := s.truss("run", "h.yaml", "--notify"); !strings.Contains(errOut, "has no notify block") {
		t.Fatalf("stderr %q", errOut)
	}
}
