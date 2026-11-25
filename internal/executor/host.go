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
	"time"
)

// tree owns every process a Step starts, so that all of them can be ended.
// The implementation is per OS: a process group on Unix, a job object on
// Windows.
type tree interface {
	// terminate asks every process of the tree to stop (SIGTERM on Unix). It
	// reports false where the OS has no such request (Windows): the caller
	// then kills at once.
	terminate() bool
	// kill force-ends every process of the tree at once. It is idempotent.
	kill()
	// afterExit runs once the shell has exited. It ends every process still
	// running and waits, bounded, for them to be gone.
	afterExit()
	// release frees the resources of the tree. Called once, last.
	release()
}

// copyGrace bounds how long Wait lets output readers drain after the tree
// is gone, in case a process escaped the tree and still holds the pipes.
const copyGrace = 2 * time.Second

// Process is a started Step command. Call Wait exactly once.
type Process struct {
	cmd         *exec.Cmd
	killTimeout time.Duration
	// exited receives the result of cmd.Wait, sent by a reaper goroutine that
	// Start launches as soon as the shell is running in its tree. It is
	// buffered (capacity 1), so the reaper never blocks and a delivered exit
	// is observable (len(exited) == 1) before Wait is called.
	exited chan error
	tree   tree
	stdout bytes.Buffer
	stderr bytes.Buffer
	reads  []*os.File
	copies sync.WaitGroup
}

// Start launches req.Run in a new host shell, inside its own process tree.
// A non-nil error means the command did not start: it is a diagnostic with
// no Exit Code and the returned *Process is nil. On success a goroutine reaps
// the shell as soon as it exits, so its exit is recorded deterministically on
// every OS; the caller must then call Wait.
func Start(ctx context.Context, req Request) (*Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	shell, args := shellCommand(req.Run)
	cmd := exec.Command(shell, args...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if req.Env != nil {
		cmd.Env = append([]string(nil), req.Env...)
	}
	prepare(cmd)

	p := &Process{cmd: cmd, killTimeout: req.KillTimeout}
	var outW, errW io.Writer = &p.stdout, &p.stderr
	if req.Out != nil {
		mu := &sync.Mutex{}
		live := &lockedWriter{mu: mu, w: req.Out}
		outW = io.MultiWriter(&p.stdout, live)
		errW = io.MultiWriter(&p.stderr, live)
		if req.ErrOut != nil {
			errW = io.MultiWriter(&p.stderr, &lockedWriter{mu: mu, w: req.ErrOut})
		}
	}
	// Own pipes (not exec's copying) so Wait returns when the shell exits,
	// even if a leftover process still holds the write ends.
	var writes []*os.File
	closeAll := func() {
		for _, f := range append(append([]*os.File(nil), p.reads...), writes...) {
			_ = f.Close()
		}
	}
	for range 2 {
		r, w, err := os.Pipe()
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("start command: %w", err)
		}
		p.reads = append(p.reads, r)
		writes = append(writes, w)
	}
	cmd.Stdout, cmd.Stderr = writes[0], writes[1]

	if err := cmd.Start(); err != nil {
		closeAll()
		return nil, fmt.Errorf("start command: %w", err)
	}
	t, err := attach(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		closeAll()
		return nil, fmt.Errorf("start command: %w", err)
	}
	p.tree = t
	for _, w := range writes {
		_ = w.Close()
	}
	for i, dst := range []io.Writer{outW, errW} {
		p.copies.Add(1)
		go func(r *os.File, dst io.Writer) {
			defer p.copies.Done()
			_, _ = io.Copy(dst, r)
		}(p.reads[i], dst)
	}
	// The reaper starts only for a Process that Start returns, so a failed
	// Start leaves no goroutine behind.
	p.exited = make(chan error, 1)
	go func() { p.exited <- cmd.Wait() }()
	return p, nil
}

// PID returns the operating-system process id of the Step's shell. On Linux and
// macOS it is also the id of the Step's process group.
func (p *Process) PID() int { return p.cmd.Process.Pid }

// DefaultKillTimeout is how long a stopped step has to end by itself before
// its processes are killed, when the Request does not say (spec 028).
const DefaultKillTimeout = 10 * time.Second

// Wait blocks until the shell exits, or ctx is done, in which case the whole
// tree is asked to stop and, after the request's KillTimeout, force-ended. Either way it ends leftover processes, waits for
// them to be gone and returns the Result.
//
// Wait must be called exactly once, and always: it collects the reaper's
// result and releases the tree. A second call would block forever.
func (p *Process) Wait(ctx context.Context) Result {
	var err error
	killed := false
	select {
	case err = <-p.exited:
	case <-ctx.Done():
		// The shell may have exited by itself just before; only a shell that
		// had not delivered its exit is ended by TRUSS.
		select {
		case err = <-p.exited:
		default:
			killed = true
			if p.killTimeout > 0 && p.tree.terminate() {
				select {
				case err = <-p.exited:
				case <-time.After(p.killTimeout):
					p.tree.kill()
					err = <-p.exited
				}
			} else {
				p.tree.kill()
				err = <-p.exited
			}
		}
	}
	p.tree.afterExit()
	p.drain()
	p.tree.release()

	// A shell that TRUSS stopped was stopped, whether it ended on SIGTERM,
	// on a code of its own after handling it (0 included), or killed.
	res := Result{Stdout: p.stdout.String(), Stderr: p.stderr.String(), Stopped: killed}
	if err == nil {
		code := 0
		res.ExitCode = &code
		return res
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		res.Err = fmt.Errorf("wait for command: %w", err)
		return res
	}
	if code := exitErr.ExitCode(); code >= 0 {
		res.ExitCode = &code
		return res
	}
	res.Err = fmt.Errorf("terminated by signal: %s", signalName(exitErr.ProcessState))
	return res
}

// drain waits for the output readers to finish, closing the pipes if a
// process outside the tree keeps them open.
func (p *Process) drain() {
	done := make(chan struct{})
	go func() { p.copies.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(copyGrace):
		for _, r := range p.reads {
			_ = r.Close()
		}
		<-done
	}
	for _, r := range p.reads {
		_ = r.Close()
	}
}

// Execute runs req.Run in a new host shell. It does not change the caller's environment.
// A command that cannot start is reported as Result.Err with no Exit Code.
func Execute(ctx context.Context, req Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	p, err := Start(ctx, req)
	if err != nil {
		return Result{Err: err}, nil
	}
	return p.Wait(ctx), nil
}

// lockedWriter serializes stdout and stderr onto one live log. Writers that
// share mu never write at the same time.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

// Shell returns the host shell path Execute uses for step commands.
func Shell() string {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			return "cmd.exe"
		}
		return shell
	}
	return "/bin/sh"
}

func shellCommand(run string) (string, []string) {
	if runtime.GOOS == "windows" {
		return Shell(), []string{"/c", run}
	}
	return Shell(), []string{"-c", run}
}
