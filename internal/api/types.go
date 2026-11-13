// Package api defines the daemon's HTTP contract: the JSON types on the wire
// and a small client for them. The CLI and TRUSS Desktop both speak it.
//
// The contract is versioned by its path prefix (/api/v1). Field names are
// snake_case and times are RFC 3339, so clients in any language can use it.
package api

import "time"

// Prefix is the path prefix of this version of the API.
const Prefix = "/api/v1"

// Run statuses.
const (
	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
	// RunSkipped marks a run that was requested but never started, for
	// example a scheduled time that passed while the machine was off.
	RunSkipped = "skipped"
)

// Step statuses.
const (
	StepPending   = "pending"
	StepRunning   = "running"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
	// StepStopped marks a step TRUSS ended because another step failed.
	StepStopped   = "stopped"
	StepCancelled = "cancelled"
	// StepSkipped marks a step that never started: its if condition was false,
	// a step it needs did not succeed, or the run ended first. Reason says
	// which when a condition or a needed step caused it.
	StepSkipped = "skipped"
)

// Triggers say why a run exists.
const (
	TriggerManual   = "manual"
	TriggerSchedule = "schedule"
	// TriggerCatchUp is a scheduled run started late, for a `missed: run` schedule.
	TriggerCatchUp = "catch-up"
	// TriggerLocal marks a run that executes in a caller's shell and only
	// reports to the daemon (`truss run --track`). The daemon never starts it.
	TriggerLocal = "local"
)

// Run is one execution of a registered pipeline, or the record of one that
// was skipped.
type Run struct {
	ID       string `json:"id"`
	Pipeline string `json:"pipeline"`
	Trigger  string `json:"trigger"`
	Status   string `json:"status"`
	// Reason explains a skipped or cancelled run, or carries the failure line.
	Reason string `json:"reason,omitempty"`
	// ScheduledFor is the time a schedule named, for scheduled runs.
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Steps        []Step     `json:"steps"`
}

// Active reports whether the run is waiting or running.
func (r Run) Active() bool { return r.Status == RunQueued || r.Status == RunRunning }

// Step is the state of one step of a run. Steps of a job are named "job/step".
type Step struct {
	ID         string     `json:"id"`
	Needs      []string   `json:"needs,omitempty"`
	Status     string     `json:"status"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Attempts is the number of the current or last attempt. It is present
	// only for a step that was run again after a failure; ExitCode, Error and
	// the times describe that attempt.
	Attempts int `json:"attempts,omitempty"`
	// ContinuedOnError marks a failed step whose continue-on-error let the run go on.
	ContinuedOnError bool `json:"continued_on_error,omitempty"`
	// Reason says why a step was skipped by a condition or by a step it needs.
	Reason string `json:"reason,omitempty"`
	// PID is the operating-system process id of the step's current or last
	// attempt, for steps the daemon started. Tracked local runs have none.
	PID int `json:"pid,omitempty"`
}

// Schedule is one cron trigger of a pipeline and when it fires next.
type Schedule struct {
	Cron     string     `json:"cron"`
	Timezone string     `json:"timezone,omitempty"`
	Missed   string     `json:"missed"`
	Next     *time.Time `json:"next,omitempty"`
}

// PlanStep is one node of a pipeline's step graph.
type PlanStep struct {
	ID    string   `json:"id"`
	Needs []string `json:"needs,omitempty"`
}

// Pipeline is a registered pipeline as the daemon currently sees its file.
type Pipeline struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Paused       bool      `json:"paused"`
	RegisteredAt time.Time `json:"registered_at"`
	// Valid is false when the file is missing or fails validation; Error says why.
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
	// DefinitionName is pipeline.name from the file.
	DefinitionName string     `json:"definition_name,omitempty"`
	Concurrency    int        `json:"concurrency,omitempty"`
	Overlap        string     `json:"overlap,omitempty"`
	Schedules      []Schedule `json:"schedules"`
	Steps          []PlanStep `json:"steps"`
	ActiveRuns     int        `json:"active_runs"`
	LastRun        *Run       `json:"last_run,omitempty"`
}

// Status describes the running daemon.
type Status struct {
	Version         string    `json:"version"`
	PID             int       `json:"pid"`
	StartedAt       time.Time `json:"started_at"`
	Home            string    `json:"home"`
	Address         string    `json:"address"`
	Pipelines       int       `json:"pipelines"`
	RunningRuns     int       `json:"running_runs"`
	QueuedRuns      int       `json:"queued_runs"`
	MaxParallelRuns int       `json:"max_parallel_runs"`
}

// RegisterRequest asks the daemon to register a definition file.
type RegisterRequest struct {
	// Path is the absolute path of the definition file.
	Path string `json:"path"`
	// Name defaults to pipeline.name from the file.
	Name string `json:"name,omitempty"`
}

// Error is the body of every non-2xx response.
type Error struct {
	Message string `json:"error"`
}

// Endpoint is what a running daemon writes to its home directory so that
// clients on the same machine, run by the same user, can reach it.
type Endpoint struct {
	Address   string    `json:"address"`
	Token     string    `json:"token"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version"`
}

// Limits of the tracking endpoints.
const (
	// MaxLogChunk is the most bytes one log append may carry.
	MaxLogChunk = 256 << 10
	// MaxTrackedSteps is the most steps a tracked run may declare.
	MaxTrackedSteps = 1000
	// MaxTrackedName is the longest pipeline name of a tracked run, in bytes.
	MaxTrackedName = 128
	// MaxOpenTracked is the most tracked runs that may be running at once.
	MaxOpenTracked = 100
	// MaxTrackedLog is the most bytes a tracked run's log may hold.
	MaxTrackedLog = 64 << 20
	// MaxReason is the longest reason a client may set, in bytes.
	MaxReason = 2000
)

// CreateRunRequest opens a tracked run: one that executes outside the daemon
// and reports its progress to it.
type CreateRunRequest struct {
	// Pipeline is the name shown for the run (pipeline.name of the file).
	Pipeline string `json:"pipeline"`
	// Steps is the step graph; every step starts as pending.
	Steps []PlanStep `json:"steps"`
}

// StepUpdate reports the state of one step of a tracked run. The daemon
// stamps the times. Status is running, succeeded, failed, stopped, cancelled or
// skipped. A failed step may be reported running again (a retry), and a
// failed step may be reported failed once more with ContinuedOnError set.
type StepUpdate struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
	// Attempts is the number of the attempt that is starting, when above 1.
	Attempts int `json:"attempts,omitempty"`
	// ContinuedOnError marks a failed step whose continue-on-error let the run go on.
	ContinuedOnError bool `json:"continued_on_error,omitempty"`
	// Reason says why a step was skipped.
	Reason string `json:"reason,omitempty"`
}

// FinishRequest closes a tracked run. Status is succeeded, failed or
// cancelled. Steps that are still open are closed with it.
type FinishRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Log response headers.
const (
	// HeaderLogOffset is the offset to ask for next.
	HeaderLogOffset = "X-Truss-Log-Offset"
	// HeaderRunStatus is the run's status when the chunk was read.
	HeaderRunStatus = "X-Truss-Run-Status"
)

// Copy returns a run that shares no memory with r.
func (r Run) Copy() Run {
	c := r
	c.Steps = make([]Step, len(r.Steps))
	copy(c.Steps, r.Steps)
	return c
}

// DirEntry is one name in a directory listing for the register dialog.
type DirEntry struct {
	Name string `json:"name"`
	// Kind is "dir" or "file". Files are only listed when they end in
	// .yaml or .yml.
	Kind string `json:"kind"`
	// Link is true when the name is a symbolic link; Kind describes its target.
	Link bool `json:"link,omitempty"`
}

// DirListing answers GET /fs. It carries names only, never file content.
type DirListing struct {
	// Path is the real (symlink-free) absolute path that was listed.
	Path string `json:"path"`
	// Parent is the directory above Path, or empty at a root.
	Parent string `json:"parent"`
	// Roots are the places to start from: "/" on Linux and macOS, the
	// existing drive roots on Windows.
	Roots   []string   `json:"roots"`
	Entries []DirEntry `json:"entries"`
	// Truncated is true when the directory held more names than were returned.
	Truncated bool `json:"truncated,omitempty"`
}

// Metrics is the daemon's view of the machine's resources (GET /metrics).
// CPU percentages are shares of the whole machine: 100 means every core busy.
// When Supported is false, Host is absent, Steps and History are empty, and
// only OS, Reason, IntervalSeconds and Daemon.PID carry values.
type Metrics struct {
	Supported bool   `json:"supported"`
	OS        string `json:"os"`
	Reason    string `json:"reason,omitempty"`
	// IntervalSeconds is how often the daemon samples.
	IntervalSeconds float64        `json:"interval_seconds"`
	SampledAt       *time.Time     `json:"sampled_at,omitempty"`
	Host            *HostMetrics   `json:"host,omitempty"`
	Daemon          ProcessMetrics `json:"daemon"`
	// Steps lists every running step the daemon started, with its processes.
	Steps []StepMetrics `json:"steps"`
	// History holds host samples, oldest first, for at most the last ten minutes.
	History []HostPoint `json:"history"`
}

// HostMetrics describes the whole machine at the last sample.
type HostMetrics struct {
	CPUPercent float64 `json:"cpu_percent"`
	// Cores holds the busy percentage of each logical core, in order.
	Cores            []float64 `json:"cores"`
	MemoryTotalBytes uint64    `json:"memory_total_bytes"`
	MemoryUsedBytes  uint64    `json:"memory_used_bytes"`
	// Load holds the 1, 5 and 15 minute load averages where the OS has them.
	Load []float64 `json:"load,omitempty"`
}

// ProcessMetrics describes a process, or a step's group of processes.
type ProcessMetrics struct {
	PID        int     `json:"pid"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   uint64  `json:"rss_bytes"`
	Threads    int     `json:"threads,omitempty"`
	// Processes counts the processes measured. Tree says whether that covers
	// everything the step started (its process group) or only its shell.
	Processes int  `json:"processes"`
	Tree      bool `json:"tree"`
}

// StepMetrics ties a step's process figures to its run.
type StepMetrics struct {
	RunID  string `json:"run_id"`
	StepID string `json:"step_id"`
	ProcessMetrics
}

// HostPoint is one host sample in Metrics.History.
type HostPoint struct {
	At              time.Time `json:"at"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryUsedBytes uint64    `json:"memory_used_bytes"`
}
