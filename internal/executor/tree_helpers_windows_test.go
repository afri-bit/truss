//go:build windows

package executor

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// pidAlive reports whether pid is a running process.
func pidAlive(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // best effort in a test helper
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
