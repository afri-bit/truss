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
	// StepSkipped marks a step that never started because the run ended first.
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
// stamps the times. Status is running, succeeded, failed, stopped or cancelled.
type StepUpdate struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
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
