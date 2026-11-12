package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/runlog"
)

// trackEvery is how often a tracked run reports to the daemon. Each report
// also tells the run whether the daemon cancelled it.
const trackEvery = 250 * time.Millisecond

// callTimeout bounds one request to the daemon. A report never holds the run
// back for longer; the run itself is not affected by a slow or dead daemon.
const callTimeout = 5 * time.Second

// tracker reports a run that executes in this process to the daemon
// (`truss run --track`). It is a best-effort observer: the run's output and
// exit code never depend on it. When the daemon cannot be reached, or has
// closed the run, tracking stops with one warning and the run carries on.
// The one request the daemon can make is a cancel, which ends the run.
//
// It buffers what the run produces and sends it from one goroutine, so no step
// waits for the network. It is an io.Writer for the run log.
type tracker struct {
	c         *api.Client
	id        string
	stderr    io.Writer
	cancelRun context.CancelFunc
	base      context.Context // carries no cancellation: a final report outlives an interrupt

	mu        sync.Mutex // guards the fields below
	buf       []byte
	steps     []api.StepUpdate
	dead      bool
	cancelled bool

	sendMu sync.Mutex // serializes reports; guards offset
	offset int64

	quit chan struct{}
	done chan struct{}
}

// newTracker opens a tracked run and starts reporting. cancelRun ends the run
// when the daemon cancels it.
func newTracker(ctx context.Context, c *api.Client, req api.CreateRunRequest, stderr io.Writer, cancelRun context.CancelFunc) (*tracker, error) {
	base := context.WithoutCancel(ctx)
	callCtx, cancel := context.WithTimeout(base, callTimeout)
	defer cancel()
	run, err := c.CreateRun(callCtx, req)
	if err != nil {
		return nil, err
	}
	t := &tracker{c: c, id: run.ID, stderr: stderr, cancelRun: cancelRun, base: base, quit: make(chan struct{}), done: make(chan struct{})}
	go t.loop()
	return t, nil
}

// Write buffers run-log text. It never fails, so the run log keeps writing to
// the terminal and the log file when the daemon is gone.
func (t *tracker) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.dead {
		t.buf = append(t.buf, p...)
	}
	return len(p), nil
}

func (t *tracker) step(u api.StepUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.dead {
		t.steps = append(t.steps, u)
	}
}

// daemonCancelled reports whether the daemon ended the run.
func (t *tracker) daemonCancelled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancelled
}

func (t *tracker) loop() {
	defer close(t.done)
	tick := time.NewTicker(trackEvery)
	defer tick.Stop()
	for {
		select {
		case <-t.quit:
			return
		case <-tick.C:
			t.flush()
		}
	}
}

// flush sends the pending step states, then the pending log text. With
// nothing to send it still asks for the run's status, which is how a cancel
// reaches a run that is quiet.
func (t *tracker) flush() {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	t.mu.Lock()
	if t.dead {
		t.mu.Unlock()
		return
	}
	steps, data := t.steps, t.buf
	t.steps, t.buf = nil, nil
	t.mu.Unlock()

	ctx, cancel := context.WithTimeout(t.base, callTimeout)
	defer cancel()
	for _, u := range steps {
		run, err := t.c.ReportStep(ctx, t.id, u)
		if err != nil {
			t.fail(ctx, err)
			return
		}
		if run.Status != api.RunRunning {
			t.lost(run.Status)
			return
		}
	}
	for first := true; first || len(data) > 0; first = false {
		n := min(len(data), api.MaxLogChunk)
		next, status, err := t.c.AppendLog(ctx, t.id, t.offset, data[:n])
		if err != nil {
			t.fail(ctx, err)
			return
		}
		t.offset, data = next, data[n:]
		if status != api.RunRunning {
			t.lost(status)
			return
		}
	}
}

// finish reports the end of the run. It sends what is still pending first.
func (t *tracker) finish(status, reason string) {
	close(t.quit)
	<-t.done
	t.flush()
	t.mu.Lock()
	dead := t.dead
	t.mu.Unlock()
	if dead {
		return
	}
	ctx, cancel := context.WithTimeout(t.base, callTimeout)
	defer cancel()
	if _, err := t.c.FinishRun(ctx, t.id, api.FinishRequest{Status: status, Reason: reason}); err != nil {
		t.fail(ctx, err)
	}
}

// lost handles a run the daemon has closed. A cancel asks this run to stop.
func (t *tracker) lost(status string) {
	t.mu.Lock()
	t.dead = true
	t.cancelled = status == api.RunCancelled
	t.mu.Unlock()
	if status == api.RunCancelled {
		fmt.Fprintf(t.stderr, "run %s was cancelled from the daemon; stopping\n", t.id)
		t.cancelRun()
		return
	}
	fmt.Fprintf(t.stderr, "warning: the daemon closed run %s as %s; it is no longer tracked and the run continues\n", t.id, status)
}

// fail handles a report that did not get through.
func (t *tracker) fail(ctx context.Context, err error) {
	var se *api.StatusError
	if errors.As(err, &se) && se.Code == 409 {
		if run, runErr := t.c.Run(ctx, t.id); runErr == nil {
			t.lost(run.Status)
			return
		}
	}
	t.mu.Lock()
	t.dead = true
	t.mu.Unlock()
	fmt.Fprintf(t.stderr, "warning: tracking stopped: %s; the run continues\n", err.Error())
}

// events forwards step events to the run log and reports step states.
type trackedEvents struct {
	log     *runlog.Log
	t       *tracker
	runCtx  context.Context
	mu      sync.Mutex
	stopped map[string]bool
}

func newTrackedEvents(log *runlog.Log, t *tracker, runCtx context.Context) *trackedEvents {
	return &trackedEvents{log: log, t: t, runCtx: runCtx, stopped: map[string]bool{}}
}

func (e *trackedEvents) StepStarted(id string) {
	e.log.StepStarted(id)
	e.t.step(api.StepUpdate{ID: id, Status: api.StepRunning})
}

func (e *trackedEvents) StepOutput(id string, chunk []byte) { e.log.StepOutput(id, chunk) }

func (e *trackedEvents) StepStopped(id, failedID string) {
	e.log.StepStopped(id, failedID)
	e.mu.Lock()
	e.stopped[id] = true
	e.mu.Unlock()
}

func (e *trackedEvents) StepFinished(id string, exitCode *int, startErr error) {
	e.log.StepFinished(id, exitCode, startErr)
	e.mu.Lock()
	stopped := e.stopped[id]
	e.mu.Unlock()
	u := api.StepUpdate{ID: id, ExitCode: exitCode}
	switch {
	case e.runCtx.Err() != nil:
		u.Status = api.StepCancelled
	case stopped:
		u.Status = api.StepStopped
	case startErr != nil:
		u.Status, u.Error, u.ExitCode = api.StepFailed, startErr.Error(), nil
	case exitCode == nil || *exitCode != 0:
		u.Status = api.StepFailed
	default:
		u.Status = api.StepSucceeded
	}
	e.t.step(u)
}
