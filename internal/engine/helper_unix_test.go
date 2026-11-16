//go:build unix

package engine

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// killSelf ends the helper process by SIGKILL, so its shell reports a signal
// death: no Exit Code, only a diagnostic. kill(2) on the own pid may return
// before the signal is delivered to the process, so the helper waits to be
// killed instead of racing it with a normal exit.
func killSelf() int {
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	time.Sleep(time.Minute)
	return 2
}

// signalStepRun is the run string for a Step that dies by a signal once the
// wait file exists. "exec" keeps the shell from reporting 128+n instead.
func signalStepRun(waitFile string) string {
	return "exec " + helperCmd("kill-self", waitFile)
}

const signalStepSupported = true

// pidAlive reports whether pid is a running process. A zombie waiting to be
// reaped counts as gone.
func pidAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return false
	}
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
			s := string(data)
			if i := strings.LastIndexByte(s, ')'); i >= 0 && len(s) > i+2 && s[i+2] == 'Z' {
				return false
			}
		}
		return true
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed arguments, numeric pid
	if err != nil {
		return false
	}
	state := strings.TrimSpace(string(out))
	return state != "" && state[0] != 'Z'
}
