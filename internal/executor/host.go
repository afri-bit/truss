package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// Execute runs req.Run in a new host shell. It does not change the caller's environment.
func Execute(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	shell, args := shellCommand(req.Run)
	cmd := exec.CommandContext(ctx, shell, args...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if req.Env != nil {
		cmd.Env = append([]string(nil), req.Env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if req.Out != nil {
		live := &lockedWriter{w: req.Out}
		cmd.Stdout = io.MultiWriter(&stdout, live)
		cmd.Stderr = io.MultiWriter(&stderr, live)
	}

	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		code := 0
		result.ExitCode = &code
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		result.ExitCode = &code
		return result, nil
	}
	result.Err = fmt.Errorf("start command: %w", err)
	return result, nil
}

// lockedWriter serializes stdout and stderr onto one live log.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func shellCommand(run string) (string, []string) {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return shell, []string{"/c", run}
	}
	return "/bin/sh", []string{"-c", run}
}
