package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Run executes a pipeline on the host, running at most the plan's concurrency
// of Steps at once. dir and env are captured by the caller and reused for every
// step. events, when set, receives each started step. A nil events value skips
// those calls and writes no transcript.
//
// A Step may add environment variables, name its own working directory and
// set a timeout; a Step that runs past its timeout fails with a diagnostic
// and no Exit Code.
//
// One scheduler goroutine owns all state. Steps start in definition order once
// every Step they need has succeeded. On the first failure the remaining
// running Steps are stopped, nothing more starts, and every started Step still
// reports its own outcome.
//
// When ctx is cancelled the run is interrupted (FR-017): the error is ctx.Err()
// itself, no later Step starts, and the returned run is partial. Callers must
// ignore the returned run whenever err is not nil.
func Run(ctx context.Context, p pipeline.Pipeline, dir string, env []string, events StepEvents) (pipeline.PipelineRun, error) {
	if err := ctx.Err(); err != nil {
		return pipeline.PipelineRun{}, err
	}
	if err := pipeline.Validate(p); err != nil {
		return pipeline.PipelineRun{}, err
	}
	plan, err := pipeline.Plan(p)
	if err != nil {
		return pipeline.PipelineRun{}, err
	}

	limit := max(plan.Concurrency, 1)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Status is only final when Run returns a nil error.
	run := pipeline.PipelineRun{Name: plan.Name, Status: pipeline.StatusSucceeded}
	type outcome struct {
		slot int
		id   string
		res  executor.Result
	}
	done := make(chan outcome)
	started := make(map[string]bool, len(plan.Graph))
	succeeded := make(map[string]bool, len(plan.Graph))
	running := 0
	failed := false

	for {
		// Start every ready Step, in definition order, while a slot is free.
		for _, step := range plan.Graph {
			if failed || ctx.Err() != nil || running >= limit {
				break
			}
			if started[step.ID] || !allDone(step.Needs, succeeded) {
				continue
			}
			started[step.ID] = true
			var out io.Writer
			if events != nil {
				events.StepStarted(step.ID)
				out = stepOutput{id: step.ID, events: events}
			}
			proc, err := executor.Start(runCtx, executor.Request{Run: step.Run, Dir: stepDir(dir, step.Dir), Env: stepEnv(env, step.Env), Out: out})
			slot := len(run.Steps)
			run.Steps = append(run.Steps, pipeline.StepRun{})
			if err != nil {
				if events != nil {
					events.StepFinished(step.ID, nil, err)
				}
				run.Steps[slot] = stepResult(step.ID, executor.Result{Err: err})
				failed = true
				run.Status = pipeline.StatusFailed
				run.FailedStep = step.ID
				cancel()
				break
			}
			running++
			go func(slot int, step pipeline.Step) {
				waitCtx := runCtx
				if step.Timeout > 0 {
					var release context.CancelFunc
					waitCtx, release = context.WithTimeout(runCtx, step.Timeout)
					defer release()
				}
				res := proc.Wait(waitCtx)
				o := outcome{slot: slot, id: step.ID, res: res}
				if res.Stopped && runCtx.Err() == nil && errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
					// The step's own limit ended it, not a failure elsewhere.
					o.res.Stopped = false
					o.res.ExitCode = nil
					o.res.Err = fmt.Errorf("timed out after %s", step.Timeout)
				}
				done <- o
			}(slot, step)
		}
		if running == 0 {
			break
		}

		o := <-done
		running--
		stepRun := stepResult(o.id, o.res)
		if failed && o.res.Stopped && ctx.Err() == nil {
			stepRun.StoppedBy = run.FailedStep
		}
		if events != nil {
			if stepRun.StoppedBy != "" {
				events.StepStopped(o.id, stepRun.StoppedBy)
			}
			events.StepFinished(o.id, o.res.ExitCode, o.res.Err)
		}
		if ctx.Err() != nil {
			// FR-017: an interrupt wins over however the Step ended. The
			// check follows the events, which may be where it was cancelled.
			continue
		}
		run.Steps[o.slot] = stepRun
		switch {
		case stepRun.Status == pipeline.StatusSucceeded:
			succeeded[o.id] = true
		case !failed:
			failed = true
			run.Status = pipeline.StatusFailed
			run.FailedStep = o.id
			cancel()
		}
	}

	if err := ctx.Err(); err != nil {
		partial := run.Steps[:0:0]
		for _, s := range run.Steps {
			if s.Status != "" {
				partial = append(partial, s)
			}
		}
		run.Steps = partial
		return run, err
	}
	return run, nil
}

// stepDir resolves a step's working directory against the run directory.
func stepDir(runDir, dir string) string {
	if dir == "" || filepath.IsAbs(dir) {
		if dir == "" {
			return runDir
		}
		return dir
	}
	return filepath.Join(runDir, dir)
}

// stepEnv returns the run environment with the step's variables appended.
// A later entry wins when the process starts, so step values override.
func stepEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	sort.Strings(names)
	env := append([]string(nil), base...)
	for _, name := range names {
		env = append(env, name+"="+extra[name])
	}
	return env
}

func allDone(ids []string, done map[string]bool) bool {
	for _, id := range ids {
		if !done[id] {
			return false
		}
	}
	return true
}

// stepOutput forwards command bytes to the step-event sink without
// adding transcript text.
type stepOutput struct {
	id     string
	events StepEvents
}

func (s stepOutput) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	chunk := make([]byte, len(p))
	copy(chunk, p)
	s.events.StepOutput(s.id, chunk)
	return len(p), nil
}

func stepResult(id string, res executor.Result) pipeline.StepRun {
	step := pipeline.StepRun{
		ID:       id,
		Status:   pipeline.StatusSucceeded,
		ExitCode: res.ExitCode,
		Output:   res.Stdout,
		Stderr:   res.Stderr,
	}
	if res.Err != nil {
		step.Status = pipeline.StatusFailed
		step.ExitCode = nil
		step.Error = res.Err.Error()
		return step
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		step.Status = pipeline.StatusFailed
	}
	return step
}
