package engine

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Run executes a pipeline sequentially on the host. dir and env are captured
// by the caller and reused for every step. log, when set, receives each step
// id and the step output as it is produced.
func Run(ctx context.Context, p pipeline.Pipeline, dir string, env []string, log io.Writer) (pipeline.PipelineRun, error) {
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
		if log != nil {
			fmt.Fprintf(log, "[%s]\n", step.ID)
		}
		res, err := executor.Execute(ctx, executor.Request{Run: step.Run, Dir: dir, Env: env, Out: log})
		if err != nil {
			return run, fmt.Errorf("execute step %s: %w", step.ID, err)
		}
		finishLogLine(log, res.Stdout, res.Stderr)
		stepRun := stepResult(step.ID, res)
		run.Steps = append(run.Steps, stepRun)
		if stepRun.Status == pipeline.StatusFailed {
			run.Status = pipeline.StatusFailed
			return run, nil
		}
	}
	return run, nil
}

func finishLogLine(log io.Writer, stdout, stderr string) {
	if log == nil || (stdout == "" && stderr == "") {
		return
	}
	if (stdout == "" || strings.HasSuffix(stdout, "\n")) && (stderr == "" || strings.HasSuffix(stderr, "\n")) {
		return
	}
	fmt.Fprintln(log)
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
