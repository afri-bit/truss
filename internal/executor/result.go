package executor

import "io"

// Request is one host-shell invocation.
// Out, when set, receives stdout and stderr as the process writes them.
type Request struct {
	Run string
	Dir string
	Env []string
	Out io.Writer
}

// Result is the outcome of running a host shell.
// ExitCode is set only when the host reports one; it is nil when the process
// could not start and when it was killed by a signal. Err then holds the
// diagnostic (for example "terminated by signal: killed"). Stopped is true
// when TRUSS ended the process tree before the shell exited and the exit was
// not a success.
type Result struct {
	ExitCode *int
	Stdout   string
	Stderr   string
	Err      error
	Stopped  bool
}
