package pipeline

// Status is the outcome of a pipeline run or a step run.
type Status string

const (
	// StatusSucceeded means the run finished with exit code 0.
	StatusSucceeded Status = "succeeded"
	// StatusFailed means the run failed or a command could not start.
	StatusFailed Status = "failed"
)

// Step is one unit of work in a pipeline definition.
type Step struct {
	ID  string
	Run string
	// Needs lists the ids of steps that must succeed before this step starts,
	// in the order they were written. It is empty when the step has none.
	Needs        []string
	runNotString bool
	needsInvalid bool
}

// Pipeline is the parsed pipeline model. Parsing it executes nothing.
type Pipeline struct {
	Version string
	Name    string
	Steps   []Step
	// Concurrency is the most steps that may run at once. 0 means the
	// definition did not declare it, which is the same as 1.
	Concurrency      int
	nonStringVersion bool
	stepsNotList     bool
	// concurrencyBad and concurrencyText record an invalid concurrency value
	// found at parse time; the text is how the value is shown in the message.
	concurrencyBad  bool
	concurrencyText string
	// unknownField is the first unknown key in document order, and
	// unknownIn says where it was found (for example "at top level").
	unknownField string
	unknownIn    string
}

// ExecutionPlan is the definition-order list of steps to run.
type ExecutionPlan struct {
	Name  string
	Steps []Step
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
}

// PipelineRun is one execution of a plan. Steps contains only started steps.
type PipelineRun struct {
	Name   string
	Status Status
	Steps  []StepRun
	Error  string
	// FailedStep is the id of the first failure the scheduler recorded. It is
	// empty when the run succeeded.
	FailedStep string
}
