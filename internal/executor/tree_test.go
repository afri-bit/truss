package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const treeDeadline = 5 * time.Second

// readPidWhenReady polls for a pid file written by the helper.
func readPidWhenReady(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(treeDeadline)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pid file %s not written within %s", path, treeDeadline)
	return 0
}

func requireGone(t *testing.T, what string, pid int) {
	t.Helper()
	deadline := time.Now().Add(treeDeadline)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%s (pid %d) is still alive after Wait returned", what, pid)
}

func waitResult(ctx context.Context, proc *Process) <-chan Result {
	ch := make(chan Result, 1)
	go func() { ch <- proc.Wait(ctx) }()
	return ch
}

func TestStop_CancelEndsShellAndGrandchild(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	proc, err := Start(context.Background(), Request{Run: helperCmd("spawn-grandchild", pidfile), Dir: dir})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := waitResult(ctx, proc)

	grandchild := readPidWhenReady(t, pidfile)
	parent := readPidWhenReady(t, pidfile+".parent")
	cancel()

	var res Result
	select {
	case res = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not return after the context was cancelled")
	}
	requireGone(t, "shell", parent)
	requireGone(t, "grandchild", grandchild)

	if !res.Stopped {
		t.Fatalf("Stopped = false, result = %+v", res)
	}
	if runtime.GOOS == "windows" {
		if res.ExitCode == nil || *res.ExitCode != 1 {
			t.Fatalf("ExitCode = %v, want 1", res.ExitCode)
		}
		return
	}
	if res.ExitCode != nil {
		t.Fatalf("ExitCode = %d, want none for a killed process", *res.ExitCode)
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "terminated by signal: killed") {
		t.Fatalf("Err = %v, want \"terminated by signal: killed\"", res.Err)
	}
}

func TestTree_LeftoverProcessIsEndedWhenShellExits(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	proc, err := Start(context.Background(), Request{Run: helperCmd("spawn-grandchild", pidfile, "leave"), Dir: dir})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	done := waitResult(context.Background(), proc)
	var res Result
	select {
	case res = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not return although the shell exited")
	}
	grandchild := readPidWhenReady(t, pidfile)
	requireGone(t, "leftover grandchild", grandchild)

	if res.Err != nil || res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("result = %+v, want exit 0", res)
	}
	if res.Stopped {
		t.Fatal("Stopped = true for a Step that exited 0 on its own")
	}
}

func TestStop_StepThatAlreadyExitedIsNotStopped(t *testing.T) {
	proc, err := Start(context.Background(), Request{Run: helperCmd("exit", "0"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	res := proc.Wait(ctx)
	cancel() // the stop arrives after the result exists
	if res.Stopped || res.Err != nil || res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("result = %+v, want succeeded and not stopped", res)
	}
}

func TestTree_ExternalSignalHasNoExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signals do not exist on Windows; the stopped Step exit is covered by TestStop_CancelEndsShellAndGrandchild")
	}
	res, err := Execute(context.Background(), Request{Run: "kill -9 $$", Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.ExitCode != nil {
		t.Fatalf("ExitCode = %d, want none (never -1)", *res.ExitCode)
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "signal") {
		t.Fatalf("Err = %v, want a signal diagnostic", res.Err)
	}
	if res.Stopped {
		t.Fatal("Stopped = true for a signal TRUSS did not send")
	}
}
