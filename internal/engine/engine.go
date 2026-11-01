package engine

import (
	"context"
	"io"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Run executes a pipeline on the host, running at most the plan's concurrency
// of Steps at once. dir and env are captured by the caller and reused for every
// step. events, when set, receives each started step. A nil events value skips
// those calls and writes no transcript.
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
	started := make(map[string]bool, len(p.Steps))
	succeeded := make(map[string]bool, len(p.Steps))
	running := 0
	failed := false

	for {
		// Start every ready Step, in definition order, while a slot is free.
		for _, step := range p.Steps {
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
			proc, err := executor.Start(runCtx, executor.Request{Run: step.Run, Dir: dir, Env: env, Out: out})
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
			go func(slot int, id string) {
				done <- outcome{slot: slot, id: id, res: proc.Wait(runCtx)}
			}(slot, step.ID)
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
