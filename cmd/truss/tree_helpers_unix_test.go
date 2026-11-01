//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// pidAlive reports whether pid is a running process. A zombie that only
// waits to be reaped counts as gone. Linux reads /proc; other Unix hosts
// (macOS has no /proc) ask ps for the process state.
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
		// ps exits non-zero when the process does not exist.
		return false
	}
	state := strings.TrimSpace(string(out))
	return state != "" && state[0] != 'Z'
}
