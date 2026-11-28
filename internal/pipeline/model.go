package pipeline

import (
	"fmt"
	"time"
)

// Status is the outcome of a pipeline run or a step run.
type Status string

const (
	// StatusSucceeded means the run finished with exit code 0.
	StatusSucceeded Status = "succeeded"
	// StatusFailed means the run failed or a command could not start.
	StatusFailed Status = "failed"
	// StatusSkipped means the step was not run because of an if: condition
	// or because a step it needs did not succeed.
	StatusSkipped Status = "skipped"
)

// Step is one unit of work in a pipeline definition.
type Step struct {
	ID  string
	Run string
	// Needs lists the ids of steps that must succeed before this step starts,
	// in the order they were written. It is empty when the step has none.
	Needs []string
	// Env holds the variables this step adds to the run environment. In an
	// ExecutionPlan it is already merged: pipeline, then job, then step.
	Env map[string]string
	// EnvSource is set by Plan. It names the layer each variable of Env comes
	// from: "project", "pipeline", "job <id>" or "step". It is nil when Env is
	// empty.
	EnvSource map[string]string
	// EnvLayers is set by Plan: the variables of each layer as written, lowest
	// first (project, flow, job, step). The engine expands ${NAME} layer by
	// layer (spec 026); Env is their merge before expansion.
	EnvLayers []map[string]string
	// Secrets names the stored secrets this step receives as environment
	// variables. In an ExecutionPlan it is already merged: pipeline, then job,
	// then step, without repeats. Only names are held here, never values.
	Secrets []string
	// Dir is the step's working directory. A relative Dir is resolved against
	// the directory the run starts in. Empty means that directory itself.
	Dir string
	// Timeout ends the step when it runs longer. Zero means no limit.
	Timeout time.Duration
	// KillTimeout is how long a stopped step may take to end after it is
	// asked to, before it is killed (spec 028). Zero means the flow's, and
	// in an ExecutionPlan the flow's is already applied; zero there means the
	// engine's default.
	KillTimeout time.Duration
	// Retries is how many more times a failed step is run. Zero means never.
	Retries int
	// RetryDelay is the wait before each retry. Zero means no wait.
	RetryDelay time.Duration
	// ContinueOnError lets the run go on when this step fails.
	ContinueOnError bool
	// If decides whether the step runs. Nil means the default: run when every
	// step it needs succeeded.
	If *Condition
	// Job, JobFirst and JobIf are set by Plan on the steps of a job: the job's
	// id, whether this is its first step, and the job's own condition.
	Job          string
	JobFirst     bool
	JobIf        *Condition
	runNotString bool
	needsInvalid bool
}

// Job is a named, ordered list of steps. The steps of a job run one after
// another; jobs wait for the jobs they need.
type Job struct {
	ID      string
	Needs   []string
	Env     map[string]string
	Secrets []string
	Dir     string
	// If decides whether the job runs. Nil means the default.
	If    *Condition
	Steps []Step
}

// Overlap says what happens when a run is requested while another run of the
// same pipeline is still active. It only matters to the daemon.
type Overlap string

const (
	// OverlapQueue starts the new run after the active one. It is the default.
	OverlapQueue Overlap = "queue"
	// OverlapSkip drops the new run.
	OverlapSkip Overlap = "skip"
	// OverlapParallel starts the new run next to the active one.
	OverlapParallel Overlap = "parallel"
)

// Missed says what a schedule does with a time that passed while nothing was
// watching the clock (the machine was off or asleep, or the daemon stopped).
type Missed string

const (
	// MissedSkip forgets the missed time. It is the default.
	MissedSkip Missed = "skip"
	// MissedRun starts one run for all the missed times together.
	MissedRun Missed = "run"
)

// Schedule is one cron trigger of a pipeline.
type Schedule struct {
	// Cron is a five-field cron expression or an @-shortcut, as written.
	Cron string
	// Timezone is an IANA zone name. Empty means the machine's local time.
	Timezone string
	Missed   Missed
}

// FileHook starts a flow when files change (on.file, spec 029).
type FileHook struct {
	// Paths are what is watched, relative to the project root or absolute:
	// a folder (everything under it), a file, or a pattern with *, ? and **.
	Paths []string
	// Ignore are patterns of changes that do not count.
	Ignore []string
	// Debounce is how long changes must stop before the flow starts.
	Debounce time.Duration
}

// RunHook starts a flow when another flow's run ends (on.run, spec 029).
type RunHook struct {
	// Flow is the name of the other flow.
	Flow string
	// Status lists the endings that start this flow: succeeded, failed or
	// any. Empty means succeeded.
	Status []string
}

// Matches reports whether a run of the hook's flow that ended with status
// (an api run status) starts the flow.
func (h RunHook) Matches(flow, status string) bool {
	if flow != h.Flow {
		return false
	}
	want := h.Status
	if len(want) == 0 {
		want = []string{"succeeded"}
	}
	for _, w := range want {
		if w == "any" && (status == "succeeded" || status == "failed" || status == "cancelled") || w == status {
			return true
		}
	}
	return false
}

// Notify says who hears about a flow's runs, and when (spec 030).
type Notify struct {
	// On lists when: failure, success, always, recovered, changed. Empty
	// means failure.
	On []string
	// Desktop shows a notification of the operating system.
	Desktop bool
	// Webhook, when set, is posted to.
	Webhook *Webhook
	// Run is a command for the host shell, run with the outcome in
	// TRUSS_NOTIFY_* variables.
	Run string
}

// Webhook is a URL to post a run's outcome to.
type Webhook struct {
	// URL is the address, or "secret:NAME" for a stored secret that holds it.
	URL string
	// Format is slack, discord, teams or generic (the default).
	Format string
}

// Pipeline is the parsed pipeline model. Parsing it executes nothing.
// A definition has either Steps or Jobs, never both.
type Pipeline struct {
	Version string
	// Name may be empty: callers default it to the file stem (spec 016).
	Name  string
	Steps []Step
	Jobs  []Job
	// Concurrency is the most steps that may run at once. 0 means the
	// definition did not declare it, which is the same as 1.
	Concurrency int
	// Overlap is empty when the definition did not declare it (queue).
	Overlap Overlap
	// Workdir is the directory steps start in, as written: relative to the
	// directory the run starts in, or absolute. Empty means that directory.
	Workdir   string
	Env       map[string]string
	Secrets   []string
	Schedules []Schedule
	// FileHook is on.file; nil when the flow has none.
	FileHook *FileHook
	// RunHooks is on.run.
	RunHooks []RunHook
	// Notify is the notify block; nil when the flow has none.
	Notify *Notify
	// KillTimeout is the flow's kill_timeout for every step that does not set
	// one (spec 028). Zero means the default.
	KillTimeout time.Duration
	// ProjectEnv is the env of the project's truss.yaml (spec 026). It is not
	// read from the flow file; the caller that knows the project sets it. It
	// is the lowest layer, below Env.
	ProjectEnv map[string]string

	nonStringVersion bool
	stepsNotList     bool
	hasSteps         bool
	hasJobs          bool
	// concurrencyBad and concurrencyText record an invalid concurrency value
	// found at parse time; the text is how the value is shown in the message.
	concurrencyBad  bool
	concurrencyText string
	// unknownField is the first unknown key in document order, and
	// unknownIn says where it was found (for example "at top level").
	unknownField string
	unknownIn    string
	// issues are schema faults found at parse time in the fields added after
	// the first schema draft, in document order. Validate reports the first.
	issues []string
}

// ExecutionPlan is what the engine runs: every step of the pipeline, with
// jobs flattened into steps named "job/step".
type ExecutionPlan struct {
	Name string
	// Steps is a valid one-at-a-time order: each step comes after its needs.
	Steps []Step
	// Graph holds the same steps in definition order. The engine starts ready
	// steps in this order.
	Graph []Step
	// Concurrency is the most steps that may run at once, at least 1.
	Concurrency int
}

// StepRun is one started step. ExitCode is nil when the process did not start.
type StepRun struct {
	ID       string
	Status   Status
	ExitCode *int
	Output   string
	Stderr   string
	Error    string
	// StoppedBy is the id of the Step whose failure caused this Step to be
	// stopped. It is empty otherwise.
	StoppedBy string
	// Attempts is how many times the step ran; the other fields describe the
	// last attempt.
	Attempts int
	// ContinuedOnError is true when the step failed and its continue-on-error
	// let the run go on.
	ContinuedOnError bool
}

// PipelineRun is one execution of a plan. Steps contains only started steps;
// a skipped step is reported through the engine's events, not here.
type PipelineRun struct {
	Name   string
	Status Status
	Steps  []StepRun
	Error  string
	// FailedStep is the id of the first failure the scheduler recorded. It is
	// empty when the run succeeded.
	FailedStep string
}

// Failure returns one line that names the pipeline, the step that failed
// first and its exit code or diagnostic. It is meant for a failed run.
func (r PipelineRun) Failure() string {
	if len(r.Steps) == 0 {
		if r.Error != "" {
			return r.Error
		}
		return r.Name + " failed"
	}
	step := r.Steps[len(r.Steps)-1]
	for _, s := range r.Steps {
		if s.ID == r.FailedStep {
			step = s
			break
		}
	}
	if step.Error != "" {
		return fmt.Sprintf("%s failed at %s: %s", r.Name, step.ID, step.Error)
	}
	code := 0
	if step.ExitCode != nil {
		code = *step.ExitCode
	}
	return fmt.Sprintf("%s failed at %s (exit %d)", r.Name, step.ID, code)
}
