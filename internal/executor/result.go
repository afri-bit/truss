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

// Result is the outcome of starting a host shell.
// ExitCode is nil and Err is set when the process could not start.
type Result struct {
	ExitCode *int
	Stdout   string
	Stderr   string
	Err      error
}
