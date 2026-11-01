//go:build unix

package executor

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// groupTree is a process group led by the shell.
type groupTree struct{ pgid int }

// prepare puts the shell in a new process group.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// attach returns the tree of a started shell; the group id is its pid.
func attach(cmd *exec.Cmd) (tree, error) {
	return &groupTree{pgid: cmd.Process.Pid}, nil
}

// kill sends SIGKILL to the whole group. A group that is already gone is fine.
func (g *groupTree) kill() {
	_ = syscall.Kill(-g.pgid, syscall.SIGKILL) // ESRCH: already gone
}

// afterExit kills what is left of the group and polls, for about a second,
// until no process of it remains.
func (g *groupTree) afterExit() {
	g.kill()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-g.pgid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (g *groupTree) release() {}

// signalName names the signal that ended the process.
func signalName(st *os.ProcessState) string {
	if ws, ok := st.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal().String()
	}
	return "unknown"
}

// endedByKill reports whether the shell was terminated by SIGKILL, the signal
// TRUSS sends to the group. killed has already been established by the
// caller.
func endedByKill(st *os.ProcessState) bool {
	ws, ok := st.Sys().(syscall.WaitStatus)
	return ok && ws.Signaled() && ws.Signal() == syscall.SIGKILL
}
