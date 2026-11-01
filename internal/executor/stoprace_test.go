package executor

import (
	"context"
	"testing"
	"time"
)

// TestStop_AlreadyExitedShellIsNotStoppedEvenWithCancelledContext covers the
// race in Wait: when the shell has already exited on its own, a context that
// is cancelled by the time Wait is called must not turn that exit into a
// Stopped Step. The Exit Code of the shell stays the result.
func TestStop_AlreadyExitedShellIsNotStoppedEvenWithCancelledContext(t *testing.T) {
	const iterations = 100
	for _, code := range []string{"0", "3"} {
		want := 0
		if code == "3" {
			want = 3
		}
		t.Run("exit "+code, func(t *testing.T) {
			dir := t.TempDir()
			for i := range iterations {
				proc, err := Start(context.Background(), Request{Run: helperCmd("exit", code), Dir: dir})
				if err != nil {
					t.Fatalf("iteration %d: Start() error = %v", i, err)
				}
				waitShellExited(t, proc)

				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				res := proc.Wait(ctx)

				if res.Stopped {
					t.Fatalf("iteration %d: Stopped = true for a shell that exited by itself", i)
				}
				if res.Err != nil {
					t.Fatalf("iteration %d: Err = %v", i, res.Err)
				}
				if res.ExitCode == nil || *res.ExitCode != want {
					t.Fatalf("iteration %d: ExitCode = %v, want %d", i, res.ExitCode, want)
				}
			}
		})
	}
}

// waitShellExited polls, with a deadline, until the shell process is no longer
// running (a zombie waiting to be reaped counts as exited).
func waitShellExited(t *testing.T, proc *Process) {
	t.Helper()
	pid := proc.cmd.Process.Pid
	deadline := time.Now().Add(treeDeadline)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("shell (pid %d) did not exit within %s", pid, treeDeadline)
}
