package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

// cond parses an if: expression for a test.
func cond(t *testing.T, text string) *pipeline.Condition {
	t.Helper()
	c, err := pipeline.ParseCondition(text)
	if err != nil {
		t.Fatalf("ParseCondition(%q) = %v", text, err)
	}
	return c
}

func runSteps(t *testing.T, concurrency int, env []string, steps ...pipeline.Step) (pipeline.PipelineRun, *eventLog) {
	t.Helper()
	rec := &eventLog{}
	p := pipeline.Pipeline{Version: "1", Name: "t", Concurrency: concurrency, Steps: steps}
	run, err := Run(context.Background(), p, t.TempDir(), env, rec)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return run, rec
}

func find(t *testing.T, run pipeline.PipelineRun, id string) pipeline.StepRun {
	t.Helper()
	for _, s := range run.Steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("step %q did not run; steps %+v", id, run.Steps)
	return pipeline.StepRun{}
}

func (r *eventLog) has(kind, id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if ev.kind == kind && ev.id == id {
			return true
		}
	}
	return false
}

func (r *eventLog) reason(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if ev.kind == "skipped" && ev.id == id {
			return ev.chunk
		}
	}
	return ""
}

func TestRun_RetryRunsAgainUntilItSucceeds(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "flaky", Run: helperCmd("flaky", counter, "2"), Retries: 3},
		pipeline.Step{ID: "after", Run: helperCmd("exit", "0"), Needs: []string{"flaky"}},
	)
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s", run.Status)
	}
	s := find(t, run, "flaky")
	if s.Attempts != 3 || s.Status != pipeline.StatusSucceeded || !strings.Contains(s.Output, "attempt 3") {
		t.Fatalf("flaky = %+v", s)
	}
	want := "started flaky,output flaky,finished flaky,retrying flaky,started flaky,output flaky,finished flaky," +
		"retrying flaky,started flaky,output flaky,finished flaky,started after,finished after"
	if got := strings.Join(rec.phases(), ","); got != want {
		t.Fatalf("events\n got %s\nwant %s", got, want)
	}
	if find(t, run, "after").Attempts != 1 {
		t.Fatalf("a step without retries ran %d times", find(t, run, "after").Attempts)
	}
}

func TestRun_RetriesExhaustedFailTheRunWithTheLastAttempt(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	run, _ := runSteps(t, 1, nil,
		pipeline.Step{ID: "flaky", Run: helperCmd("flaky", counter, "9"), Retries: 1},
		pipeline.Step{ID: "after", Run: helperCmd("exit", "0"), Needs: []string{"flaky"}},
	)
	if run.Status != pipeline.StatusFailed || run.FailedStep != "flaky" {
		t.Fatalf("run = %+v", run)
	}
	s := find(t, run, "flaky")
	if s.Attempts != 2 || s.ExitCode == nil || *s.ExitCode != 1 || !strings.Contains(s.Output, "attempt 2") {
		t.Fatalf("flaky = %+v", s)
	}
	if len(run.Steps) != 1 {
		t.Fatalf("a later step ran: %+v", run.Steps)
	}
}

func TestRun_RetryDelayIsWaited(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	begin := time.Now()
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "flaky", Run: helperCmd("flaky", counter, "1"), Retries: 1, RetryDelay: 300 * time.Millisecond})
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s", run.Status)
	}
	if elapsed := time.Since(begin); elapsed < 250*time.Millisecond {
		t.Fatalf("elapsed %s, want at least the retry delay", elapsed)
	}
	if !rec.has("retrying", "flaky") {
		t.Fatalf("no retrying event: %q", rec.phases())
	}
}

func TestRun_StepWithoutRetriesIsNotRetried(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	run, rec := runSteps(t, 1, nil, pipeline.Step{ID: "flaky", Run: helperCmd("flaky", counter, "1")})
	if run.Status != pipeline.StatusFailed || find(t, run, "flaky").Attempts != 1 || rec.has("retrying", "flaky") {
		t.Fatalf("run = %+v, events %q", run, rec.phases())
	}
}

func TestRun_TimeoutFailsTheAttemptAndIsRetried(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	// The first attempt blocks until its timeout; the second finds the counter
	// file already there and exits at once.
	run, _ := runSteps(t, 1, nil,
		pipeline.Step{ID: "slow", Run: helperCmd("block-first", counter), Timeout: 2 * time.Second, Retries: 1})
	s := find(t, run, "slow")
	if run.Status != pipeline.StatusSucceeded || s.Attempts != 2 {
		t.Fatalf("run = %+v", run)
	}
}

func TestRun_RetryWaitIsEndedWhenAnotherStepFailsTheRun(t *testing.T) {
	// b fails only once a is waiting to retry, so a's wait is what ends.
	marker := filepath.Join(t.TempDir(), "a-waiting")
	rec := &touchOnRetry{eventLog: &eventLog{}, path: marker}
	begin := time.Now()
	p := pipeline.Pipeline{Version: "1", Name: "t", Concurrency: 2, Steps: []pipeline.Step{
		{ID: "a", Run: helperCmd("exit", "1"), Retries: 3, RetryDelay: time.Hour},
		{ID: "b", Run: helperCmd("exit-after", marker, "1")},
	}}
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(begin) > 30*time.Second {
		t.Fatal("the retry wait was not ended")
	}
	if run.Status != pipeline.StatusFailed || run.FailedStep != "b" {
		t.Fatalf("run = %+v", run)
	}
	a := find(t, run, "a")
	if a.StoppedBy != "b" || a.Attempts != 1 || a.ExitCode == nil || *a.ExitCode != 1 {
		t.Fatalf("a = %+v", a)
	}
	if !rec.has("stopped", "a") {
		t.Fatalf("no stopped event for a: %q", rec.phases())
	}
}

// touchOnRetry creates a file when a step is about to retry.
type touchOnRetry struct {
	*eventLog
	path string
}

func (c *touchOnRetry) StepRetrying(id string, attempt, attempts int, delay time.Duration) {
	c.eventLog.StepRetrying(id, attempt, attempts, delay)
	_ = os.WriteFile(c.path, nil, 0o600)
}

func TestRun_StoppedStepWithRetriesIsNotRunAgain(t *testing.T) {
	run, rec := runSteps(t, 2, nil,
		pipeline.Step{ID: "a", Run: helperCmd("block", filepath.Join(t.TempDir(), "pid")), Retries: 2},
		pipeline.Step{ID: "b", Run: helperCmd("exit", "1")},
	)
	a := find(t, run, "a")
	if run.FailedStep != "b" || a.StoppedBy != "b" || a.Attempts != 1 || rec.has("retrying", "a") {
		t.Fatalf("a = %+v, events %q", a, rec.phases())
	}
}

func TestRun_ContinueOnErrorAfterTheRunFailedIsNotLoggedAsContinued(t *testing.T) {
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "build", Run: helperCmd("exit", "1")},
		pipeline.Step{ID: "late", Run: helperCmd("exit", "1"), Needs: []string{"build"}, ContinueOnError: true, If: cond(t, "always()")},
	)
	if run.Status != pipeline.StatusFailed || run.FailedStep != "build" || find(t, run, "late").ContinuedOnError || rec.has("continued", "late") {
		t.Fatalf("run = %+v, events %q", run, rec.phases())
	}
}

func TestRun_JobThatDidNotStartSkipsAllItsSteps(t *testing.T) {
	rec := &eventLog{}
	p := jobPlan(t,
		pipeline.Job{ID: "build", Steps: []pipeline.Step{{ID: "compile", Run: helperCmd("exit", "1")}}},
		pipeline.Job{ID: "deploy", Needs: []string{"build"}, Steps: []pipeline.Step{
			{ID: "push", Run: helperCmd("exit", "0")},
			{ID: "logout", Run: helperCmd("exit", "0"), If: cond(t, "always()")},
		}},
	)
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Steps) != 1 || run.FailedStep != "build/compile" {
		t.Fatalf("a step of a job that did not start ran: %+v", run.Steps)
	}
	if rec.has("skipped", "deploy/push") || rec.has("skipped", "deploy/logout") {
		t.Fatalf("fail-fast skips are silent: %q", rec.phases())
	}

	// Without a failure the job is skipped because a job it needs was.
	p = jobPlan(t,
		pipeline.Job{ID: "build", If: cond(t, "env.NOPE == 'x'"), Steps: []pipeline.Step{{ID: "compile", Run: helperCmd("exit", "0")}}},
		pipeline.Job{ID: "deploy", Needs: []string{"build"}, Steps: []pipeline.Step{
			{ID: "push", Run: helperCmd("exit", "0")},
			{ID: "logout", Run: helperCmd("exit", "0"), If: cond(t, "always()")},
		}},
	)
	rec = &eventLog{}
	run, err = Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil || len(run.Steps) != 0 || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if rec.reason("deploy/push") != "step build/compile did not succeed" || rec.reason("deploy/logout") != `job "deploy" did not start` {
		t.Fatalf("events %q", rec.phases())
	}
}

func TestRun_ContinueOnErrorKeepsTheRunGoing(t *testing.T) {
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "lint", Run: helperCmd("exit", "1"), ContinueOnError: true},
		pipeline.Step{ID: "test", Run: helperCmd("exit", "0")},
		pipeline.Step{ID: "report", Run: helperCmd("exit", "0"), Needs: []string{"lint"}},
	)
	if run.Status != pipeline.StatusSucceeded || run.FailedStep != "" {
		t.Fatalf("run = %+v", run)
	}
	lint := find(t, run, "lint")
	if lint.Status != pipeline.StatusFailed || !lint.ContinuedOnError {
		t.Fatalf("lint = %+v", lint)
	}
	find(t, run, "test")
	if !rec.has("continued", "lint") {
		t.Fatalf("no continued event: %q", rec.phases())
	}
	// A step that needs the failed step does not run (the default condition).
	if len(run.Steps) != 2 || rec.reason("report") != "step lint did not succeed" {
		t.Fatalf("steps %+v, reason %q", run.Steps, rec.reason("report"))
	}
}

func TestRun_ContinueOnErrorDoesNotStopRunningSteps(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "failed")
	run, _ := runSteps(t, 2, nil,
		pipeline.Step{ID: "flaky", Run: helperCmd("touch-exit", marker, "1"), ContinueOnError: true},
		pipeline.Step{ID: "slow", Run: helperCmd("exit-after", marker, "0")},
	)
	if run.Status != pipeline.StatusSucceeded || find(t, run, "slow").Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v", run)
	}
}

func TestRun_ContinueOnErrorAppliesToAStartFailure(t *testing.T) {
	rec := &eventLog{}
	p := pipeline.Pipeline{Version: "1", Name: "t", Steps: []pipeline.Step{
		{ID: "bad", Run: helperCmd("exit", "0"), Dir: "no-such-dir", ContinueOnError: true},
		{ID: "ok", Run: helperCmd("exit", "0")},
	}}
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	bad := find(t, run, "bad")
	if run.Status != pipeline.StatusSucceeded || bad.Error == "" || bad.ExitCode != nil || !bad.ContinuedOnError {
		t.Fatalf("run = %+v", run)
	}
	find(t, run, "ok")
}

func TestRun_FailureStepsAndAlwaysStepsRunAfterAFailure(t *testing.T) {
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "build", Run: helperCmd("exit", "1")},
		pipeline.Step{ID: "test", Run: helperCmd("exit", "0"), Needs: []string{"build"}},
		pipeline.Step{ID: "notify", Run: helperCmd("exit", "0"), Needs: []string{"build"}, If: cond(t, "failure()")},
		pipeline.Step{ID: "cleanup", Run: helperCmd("exit", "0"), Needs: []string{"test", "notify"}, If: cond(t, "always()")},
		pipeline.Step{ID: "publish", Run: helperCmd("exit", "0"), Needs: []string{"test"}, If: cond(t, "success()")},
	)
	if run.Status != pipeline.StatusFailed || run.FailedStep != "build" {
		t.Fatalf("run = %+v", run)
	}
	find(t, run, "notify")
	find(t, run, "cleanup")
	for _, id := range []string{"test", "publish"} {
		for _, s := range run.Steps {
			if s.ID == id {
				t.Fatalf("%s ran after the failure", id)
			}
		}
	}
	// test and publish never started only because the run failed: no skipped
	// event, as before conditions existed. success() does not bypass fail-fast.
	if rec.has("skipped", "test") || rec.has("skipped", "publish") {
		t.Fatalf("a fail-fast step was reported as skipped: %q", rec.phases())
	}
}

func TestRun_AlwaysStepStartedAfterAFailureIsNotStoppedByIt(t *testing.T) {
	// cleanup starts after build failed and the run's stop signal was sent.
	// It must still run to its end.
	marker := filepath.Join(t.TempDir(), "cleaned")
	run, _ := runSteps(t, 2, nil,
		pipeline.Step{ID: "build", Run: helperCmd("exit", "1")},
		pipeline.Step{ID: "cleanup", Run: helperCmd("touch-exit", marker, "0"), Needs: []string{"build"}, If: cond(t, "always()")},
	)
	c := find(t, run, "cleanup")
	if run.Status != pipeline.StatusFailed || c.Status != pipeline.StatusSucceeded || c.StoppedBy != "" {
		t.Fatalf("run = %+v", run)
	}
}

func TestRun_AlwaysStepWithoutNeedsIsStoppedLikeAnyRunningStep(t *testing.T) {
	// A step with no needs starts at once, so a later failure stops it. Order
	// it with needs to have it run last.
	run, _ := runSteps(t, 2, nil,
		pipeline.Step{ID: "build", Run: helperCmd("exit", "1")},
		pipeline.Step{ID: "cleanup", Run: helperCmd("block", filepath.Join(t.TempDir(), "pid")), If: cond(t, "always()")},
	)
	if c := find(t, run, "cleanup"); c.StoppedBy != "build" {
		t.Fatalf("cleanup = %+v", c)
	}
}

func TestRun_EnvConditions(t *testing.T) {
	steps := []pipeline.Step{
		{ID: "prod", Run: helperCmd("exit", "0"), If: cond(t, "env.STAGE == 'prod'")},
		{ID: "notprod", Run: helperCmd("exit", "0"), If: cond(t, "env.STAGE != 'prod'")},
		{ID: "own", Run: helperCmd("exit", "0"), Env: map[string]string{"STAGE": "dev"}, If: cond(t, "env.STAGE == 'dev'")},
		{ID: "unset", Run: helperCmd("exit", "0"), If: cond(t, "env.NOPE_NOT_SET == ''")},
	}
	run, rec := runSteps(t, 1, []string{"STAGE=prod"}, steps...)
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s", run.Status)
	}
	find(t, run, "prod")
	find(t, run, "own")
	find(t, run, "unset")
	if rec.reason("notprod") != "its if condition is false" || len(run.Steps) != 3 {
		t.Fatalf("steps %+v, reason %q", run.Steps, rec.reason("notprod"))
	}
}

func TestRun_SkippedStepSkipsWhatNeedsIt(t *testing.T) {
	run, rec := runSteps(t, 1, nil,
		pipeline.Step{ID: "a", Run: helperCmd("exit", "0"), If: cond(t, "env.GO == 'yes'")},
		pipeline.Step{ID: "b", Run: helperCmd("exit", "0"), Needs: []string{"a"}},
		pipeline.Step{ID: "c", Run: helperCmd("exit", "0"), Needs: []string{"a"}, If: cond(t, "always()")},
	)
	if run.Status != pipeline.StatusSucceeded || len(run.Steps) != 1 || run.Steps[0].ID != "c" {
		t.Fatalf("run = %+v", run)
	}
	if rec.reason("b") != "step a did not succeed" || rec.reason("a") == "" {
		t.Fatalf("events %q", rec.phases())
	}
}

func jobPlan(t *testing.T, jobs ...pipeline.Job) pipeline.Pipeline {
	t.Helper()
	return pipeline.Pipeline{Version: "1", Name: "t", Jobs: jobs}
}

func TestRun_JobConditionSkipsEveryStepOfTheJob(t *testing.T) {
	rec := &eventLog{}
	p := jobPlan(t,
		pipeline.Job{ID: "build", Steps: []pipeline.Step{{ID: "one", Run: helperCmd("exit", "0")}}},
		pipeline.Job{ID: "deploy", Needs: []string{"build"}, If: cond(t, "env.DEPLOY == 'yes'"), Steps: []pipeline.Step{
			{ID: "one", Run: helperCmd("exit", "0")},
			{ID: "two", Run: helperCmd("exit", "0"), If: cond(t, "always()")},
		}},
	)
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != pipeline.StatusSucceeded || len(run.Steps) != 1 {
		t.Fatalf("run = %+v", run)
	}
	if !rec.has("skipped", "deploy/one") || !rec.has("skipped", "deploy/two") || !strings.Contains(rec.reason("deploy/two"), `job "deploy"`) {
		t.Fatalf("events %q", rec.phases())
	}
	// The same job runs when the condition holds.
	run, err = Run(context.Background(), p, t.TempDir(), []string{"DEPLOY=yes"}, nil)
	if err != nil || len(run.Steps) != 3 {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
}

func TestRun_CleanupJobRunsAfterAFailedJob(t *testing.T) {
	p := jobPlan(t,
		pipeline.Job{ID: "build", Steps: []pipeline.Step{{ID: "compile", Run: helperCmd("exit", "1")}}},
		pipeline.Job{ID: "cleanup", Needs: []string{"build"}, If: cond(t, "always()"), Steps: []pipeline.Step{
			{ID: "one", Run: helperCmd("exit", "0")},
			{ID: "two", Run: helperCmd("exit", "0")},
		}},
		pipeline.Job{ID: "report", Needs: []string{"build"}, Steps: []pipeline.Step{{ID: "one", Run: helperCmd("exit", "0")}}},
	)
	run, err := Run(context.Background(), p, t.TempDir(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, s := range run.Steps {
		ids = append(ids, s.ID)
	}
	if run.Status != pipeline.StatusFailed || run.FailedStep != "build/compile" || strings.Join(ids, ",") != "build/compile,cleanup/one,cleanup/two" {
		t.Fatalf("run = %+v, ran %v", run, ids)
	}
}

func TestRun_InterruptDuringRetryWaitReturnsTheContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rec := &cancelOnRetry{eventLog: &eventLog{}, cancel: cancel}
	p := pipeline.Pipeline{Version: "1", Name: "t", Steps: []pipeline.Step{
		{ID: "a", Run: helperCmd("exit", "1"), Retries: 2, RetryDelay: time.Hour}}}
	_, err := Run(ctx, p, t.TempDir(), nil, rec)
	if err != context.Canceled {
		t.Fatalf("err = %v", err)
	}
}

type cancelOnRetry struct {
	*eventLog
	cancel context.CancelFunc
}

func (c *cancelOnRetry) StepRetrying(id string, attempt, attempts int, delay time.Duration) {
	c.eventLog.StepRetrying(id, attempt, attempts, delay)
	c.cancel()
}
