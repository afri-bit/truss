package executor

import (
	"io"
	"time"
)

// Request is one host-shell invocation.
// Out, when set, receives stdout and stderr as the process writes them. When
// ErrOut is also set, stderr goes to ErrOut instead, so a caller can treat the
// streams separately; writes to the two are still serialized with each other.
type Request struct {
	Run    string
	Dir    string
	Env    []string
	Out    io.Writer
	ErrOut io.Writer
	// KillTimeout is how long a stopped step may take to end after it was
	// asked to (SIGTERM on Unix) before it is killed. 0 kills at once
	// (spec 028).
	KillTimeout time.Duration
	// KillNow, when closed, ends the wait of KillTimeout at once: a second
	// Ctrl-C (spec 028).
	KillNow <-chan struct{}
}

// Result is the outcome of running a host shell.
// ExitCode is set only when the host reports one; it is nil when the process
// could not start and when it was killed by a signal. Err then holds the
// diagnostic (for example "terminated by signal: killed"). Stopped is true
// when TRUSS ended, or asked to end, the process tree before the shell
// exited, whatever the exit (spec 028).
type Result struct {
	ExitCode *int
	Stdout   string
	Stderr   string
	Err      error
	Stopped  bool
}
