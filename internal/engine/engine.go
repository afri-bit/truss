package engine

import (
	"context"
	"fmt"
	"io"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Run executes a pipeline sequentially on the host. dir and env are captured
// by the caller and reused for every step. events, when set, receives each
// started step. A nil events value skips those calls and writes no transcript.
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

	run := pipeline.PipelineRun{Name: plan.Name, Status: pipeline.StatusSucceeded}
	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return run, err
		}
		var out io.Writer
		if events != nil {
			events.StepStarted(step.ID)
			out = stepOutput{id: step.ID, events: events}
		}
		res, err := executor.Execute(ctx, executor.Request{Run: step.Run, Dir: dir, Env: env, Out: out})
		if err != nil {
			if events != nil {
				events.StepFinished(step.ID, nil, err)
			}
			return run, fmt.Errorf("execute step %s: %w", step.ID, err)
		}
		if events != nil {
			events.StepFinished(step.ID, res.ExitCode, res.Err)
		}
		stepRun := stepResult(step.ID, res)
		run.Steps = append(run.Steps, stepRun)
		if stepRun.Status == pipeline.StatusFailed {
			run.Status = pipeline.StatusFailed
			return run, nil
		}
	}
	return run, nil
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
