//go:build unix

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// moduleRoot returns the directory of go.mod, found from this source file.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source file")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// killPidFile force-ends the process in a pid file, if there is one.
func killPidFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func TestMain_InterruptEndsStepsAndKeepsLog(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "truss")
	build := exec.Command("go", "build", "-o", bin, "./cmd/truss") //nolint:gosec // fixed arguments
	build.Dir = moduleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	for _, tc := range []struct {
		name string
		sig  syscall.Signal
	}{
		{"SIGINT", syscall.SIGINT},
		{"SIGTERM", syscall.SIGTERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pidfile := filepath.Join(dir, "child.pid")
			logPath, previous := writeBlockingPipeline(t, dir, pidfile)

			var stdout, stderr bytes.Buffer
			cmd := exec.Command(bin, "run", "pipeline.yaml") //nolint:gosec // the binary built above
			cmd.Dir = dir
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			cmd.WaitDelay = 5 * time.Second
			if err := cmd.Start(); err != nil {
				t.Fatalf("start truss: %v", err)
			}
			// Wait's error is not needed: the exit code is checked from ProcessState below.
			exited := make(chan struct{})
			go func() { _ = cmd.Wait(); close(exited) }()
			// Never leave processes behind, whatever the outcome.
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				killPidFile(pidfile)
				killPidFile(pidfile + ".parent")
			})

			grandchild := readPidWhenReady(t, pidfile)
			parent := readPidWhenReady(t, pidfile+".parent")
			if err := cmd.Process.Signal(tc.sig); err != nil {
				t.Fatalf("send %s: %v", tc.name, err)
			}

			select {
			case <-exited:
			case <-time.After(interruptDeadline):
				t.Fatalf("truss did not exit within %s after %s", interruptDeadline, tc.name)
			}
			if code := cmd.ProcessState.ExitCode(); code != 1 {
				t.Fatalf("exit code = %d (%v), want 1; stderr = %s", code, cmd.ProcessState, stderr.String())
			}
			errOut := stderr.String()
			if !strings.Contains(errOut, "error:") || !strings.Contains(errOut, "context canceled") {
				t.Fatalf("stderr = %q, want error: ... context canceled", errOut)
			}
			if all := strings.ToLower(withoutSetupDirectory(stdout.String())); strings.Contains(all, "cancelled") {
				t.Fatalf("stdout reports a cancelled status:\n%s", stdout.String())
			}
			requireGone(t, "grandchild", grandchild)
			requireGone(t, "step process", parent)
			assertLogUntouched(t, logPath, previous)
		})
	}
}
