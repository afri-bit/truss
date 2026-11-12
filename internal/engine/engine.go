package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

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
// every Step they need has settled. A Step with no if: condition starts only
// when every Step it needs succeeded; otherwise it is skipped. On the first
// failure that is not covered by continue-on-error the remaining running Steps
// are stopped and nothing more starts, except Steps whose condition calls
// always() or failure(). Every started Step still reports its own outcome.
//
// A failed attempt of a Step with retries is run again after the Step's retry
// delay. The Step's result is its last attempt.
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
	// outcome reports one finished attempt, or (due) that a retry delay is over.
	type outcome struct {
		slot    int
		attempt int
		id      string
		res     executor.Result
		due     bool // the retry delay elapsed; res is the previous attempt
		aborted bool // the wait was cut short; res is the previous attempt
		// afterFailure is true when the run had already failed as the retry
		// was scheduled, so a later failure is not a reason to drop it.
		afterFailure bool
	}
	done := make(chan outcome)
	byID := make(map[string]pipeline.Step, len(plan.Graph))
	for _, s := range plan.Graph {
		byID[s.ID] = s
	}
	started := make(map[string]bool, len(plan.Graph))
	settled := make(map[string]pipeline.Status, len(plan.Graph))
	// jobSkipped holds the jobs that will not run, with the reason the rest of
	// their steps are skipped for.
	type skip struct {
		reason string
		silent bool
	}
	jobSkipped := map[string]skip{}
	running := 0 // steps with a process or a pending retry
	failed := false

	// bypass reports whether the step may start after the run has failed.
	bypass := func(step pipeline.Step) bool { return step.If.AfterFailure() || step.JobIf.AfterFailure() }
	// holds evaluates a condition against the steps the step needs.
	holds := func(c *pipeline.Condition, step pipeline.Step) bool {
		ec := pipeline.EvalContext{Success: true, Env: envLookup(stepEnv(env, step.Env))}
		for _, need := range step.Needs {
			switch settled[need] {
			case pipeline.StatusSucceeded:
			case pipeline.StatusFailed:
				ec.Success, ec.Failure = false, true
			default:
				ec.Success = false
			}
		}
		return c.Eval(ec)
	}
	// decide says whether a step whose needs have settled runs. When it does
	// not, reason says why and silent is set for a step that only never
	// started because the run had already failed, as before conditions existed.
	decide := func(step pipeline.Step) (runIt bool, reason string, silent bool) {
		if js, ok := jobSkipped[step.Job]; ok && step.Job != "" {
			return false, js.reason, js.silent
		}
		// skipFirst records that a job never started, so that none of its
		// steps run, whatever their own conditions say.
		skipFirst := func(reason string, silent bool) (bool, string, bool) {
			if step.JobFirst {
				jobSkipped[step.Job] = skip{reason: fmt.Sprintf("job %q did not start", step.Job), silent: silent}
			}
			return false, reason, silent
		}
		if failed && !bypass(step) {
			return skipFirst("", true)
		}
		gate := true
		if step.JobFirst && step.JobIf != nil {
			if !holds(step.JobIf, step) {
				jobSkipped[step.Job] = skip{reason: fmt.Sprintf("the if condition of job %q is false", step.Job)}
				return false, jobSkipped[step.Job].reason, false
			}
			gate = false // the job's condition replaces the default for its first step
		}
		if step.If != nil {
			if holds(step.If, step) {
				return true, "", false
			}
			return false, "its if condition is false", false
		}
		if gate {
			for _, need := range step.Needs {
				if settled[need] != pipeline.StatusSucceeded {
					return skipFirst(fmt.Sprintf("step %s did not succeed", need), false)
				}
			}
		}
		return true, "", false
	}

	// finish handles one finished attempt. It reports true when a retry is
	// pending, so the step still occupies a slot.
	finish := func(o outcome) bool {
		step := byID[o.id]
		stepRun := stepResult(o.id, o.res)
		stepRun.Attempts = o.attempt
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
			return false
		}
		if stepRun.Status == pipeline.StatusFailed && stepRun.StoppedBy == "" && !o.res.Stopped &&
			o.attempt <= step.Retries && (!failed || bypass(step)) {
			next := o.attempt + 1
			if events != nil {
				events.StepRetrying(o.id, next, step.Retries+1, step.RetryDelay)
			}
			waitCtx := runCtx
			scheduledAfterFailure := failed
			if failed {
				waitCtx = ctx
			}
			go func() {
				r := outcome{slot: o.slot, attempt: next, id: o.id, res: o.res, due: true, afterFailure: scheduledAfterFailure}
				if step.RetryDelay > 0 {
					timer := time.NewTimer(step.RetryDelay)
					select {
					case <-timer.C:
					case <-waitCtx.Done():
						r.aborted = true
					}
					timer.Stop()
				}
				done <- r
			}()
			return true
		}
		if stepRun.Status == pipeline.StatusSucceeded {
			run.Steps[o.slot] = stepRun
			settled[o.id] = pipeline.StatusSucceeded
			return false
		}
		settled[o.id] = pipeline.StatusFailed
		if step.ContinueOnError && stepRun.StoppedBy == "" && !failed {
			stepRun.ContinuedOnError = true
			run.Steps[o.slot] = stepRun
			if events != nil {
				events.StepContinued(o.id)
			}
			return false
		}
		run.Steps[o.slot] = stepRun
		if !failed {
			failed = true
			run.Status = pipeline.StatusFailed
			run.FailedStep = o.id
			cancel()
		}
		return false
	}

	// abandon ends a Step whose retry wait was cut short by a failure elsewhere.
	abandon := func(o outcome) {
		stepRun := stepResult(o.id, o.res)
		stepRun.Attempts = o.attempt - 1
		stepRun.StoppedBy = run.FailedStep
		if events != nil {
			events.StepStopped(o.id, run.FailedStep)
		}
		run.Steps[o.slot] = stepRun
		settled[o.id] = pipeline.StatusFailed
	}

	// startAttempt starts one attempt of a step. It reports true when a
	// process is running, and false when the attempt already ended.
	startAttempt := func(step pipeline.Step, slot, attempt int) bool {
		// A step that may start after a failure must outlive the stop signal.
		base := runCtx
		if failed {
			base = ctx
		}
		var out io.Writer
		if events != nil {
			events.StepStarted(step.ID)
			out = stepOutput{id: step.ID, events: events}
		}
		proc, err := executor.Start(base, executor.Request{Run: step.Run, Dir: stepDir(dir, step.Dir), Env: stepEnv(env, step.Env), Out: out})
		if err != nil {
			return finish(outcome{slot: slot, attempt: attempt, id: step.ID, res: executor.Result{Err: err}})
		}
		go func() {
			waitCtx := base
			if step.Timeout > 0 {
				var release context.CancelFunc
				waitCtx, release = context.WithTimeout(base, step.Timeout)
				defer release()
			}
			res := proc.Wait(waitCtx)
			o := outcome{slot: slot, attempt: attempt, id: step.ID, res: res}
			if res.Stopped && base.Err() == nil && errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				// The step's own limit ended it, not a failure elsewhere.
				o.res.Stopped = false
				o.res.ExitCode = nil
				o.res.Err = fmt.Errorf("timed out after %s", step.Timeout)
			}
			done <- o
		}()
		return true
	}

	// schedule settles every step that can be skipped and starts every ready
	// step, in definition order, while a slot is free.
	schedule := func() {
		for changed := true; changed; {
			changed = false
			for _, step := range plan.Graph {
				if ctx.Err() != nil {
					return
				}
				if started[step.ID] || !allSettled(step.Needs, settled) {
					continue
				}
				runIt, reason, silent := decide(step)
				if !runIt {
					started[step.ID] = true
					settled[step.ID] = pipeline.StatusSkipped
					if events != nil && !silent {
						events.StepSkipped(step.ID, reason)
					}
					changed = true
					continue
				}
				if running >= limit {
					continue
				}
				started[step.ID] = true
				changed = true
				slot := len(run.Steps)
				run.Steps = append(run.Steps, pipeline.StepRun{})
				running++
				if !startAttempt(step, slot, 1) {
					running--
				}
			}
		}
	}

	for {
		schedule()
		if running == 0 {
			break
		}
		o := <-done
		switch {
		case !o.due:
			if !finish(o) {
				running--
			}
		case ctx.Err() != nil:
			running-- // interrupted while waiting to retry
		case o.aborted || (failed && !o.afterFailure):
			abandon(o)
			running--
		default:
			if !startAttempt(byID[o.id], o.slot, o.attempt) {
				running--
			}
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

func allSettled(ids []string, settled map[string]pipeline.Status) bool {
	for _, id := range ids {
		if settled[id] == "" {
			return false
		}
	}
	return true
}

// envLookup returns a function that finds a variable in env, where a later
// entry wins. Names are case-insensitive on Windows, as the host treats them.
func envLookup(env []string) func(string) string {
	return func(name string) string {
		for i := len(env) - 1; i >= 0; i-- {
			k, v, ok := strings.Cut(env[i], "=")
			if ok && (k == name || runtime.GOOS == "windows" && strings.EqualFold(k, name)) {
				return v
			}
		}
		return ""
	}
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
