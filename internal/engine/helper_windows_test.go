//go:build windows

package engine

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// killSelf is not used on Windows: it has no signals, so the "ended by a
// signal" case is covered on Unix only.
func killSelf() int { return 2 }

func signalStepRun(string) string { return "" }

const signalStepSupported = false

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
