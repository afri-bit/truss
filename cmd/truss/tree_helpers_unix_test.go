//go:build unix

package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// pidAlive reports whether pid is a running process. A zombie that only
// waits to be reaped counts as gone.
func pidAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return false
	}
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		s := string(data)
		if i := strings.LastIndexByte(s, ')'); i >= 0 && len(s) > i+2 && s[i+2] == 'Z' {
			return false
		}
	}
	return true
}
