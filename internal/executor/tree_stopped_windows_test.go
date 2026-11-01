//go:build windows

package executor

// wantStoppedExitCode is the exit code of a Step ended by TRUSS.
func wantStoppedExitCode() int { return stoppedExitCode }
