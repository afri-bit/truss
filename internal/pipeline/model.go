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
	ID           string
	Run          string
	runNotString bool
}

// Pipeline is the parsed pipeline model. Parsing it executes nothing.
type Pipeline struct {
	Version          string
	Name             string
	Steps            []Step
	nonStringVersion bool
	stepsNotList     bool
}

// ExecutionPlan is the definition-order list of steps to run.
type ExecutionPlan struct {
	Name  string
	Steps []Step
}

// StepRun is one started step. ExitCode is nil when the process did not start.
type StepRun struct {
	ID       string
	Status   Status
	ExitCode *int
	Output   string
	Stderr   string
	Error    string
}

// PipelineRun is one execution of a plan. Steps contains only started steps.
type PipelineRun struct {
	Name   string
	Status Status
	Steps  []StepRun
	Error  string
}
