//go:build !windows

package executor

// wantStoppedExitCode is unused on Unix: a stopped Step has no exit code there.
func wantStoppedExitCode() int { return -1 }
