package engine

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

// RunInfo describes a run for the variables TRUSS sets in every step
// (spec 026). Empty fields are left out, except Flow, which defaults to the
// flow's name.
type RunInfo struct {
	// ID is the run id: the daemon's, or the one a local run makes up.
	ID string
	// Flow is the name the run is known by (the registered name for the
	// daemon). TRUSS_PIPELINE, its older name, gets the same value.
	Flow string
	// Trigger is what started the run: manual, schedule, local, ...
	Trigger string
	// ProjectDir is the root of the flow's project, or the run's directory
	// when the flow is not in a project.
	ProjectDir string
}

// WithRun names the run for the built-in variables.
func WithRun(info RunInfo) Option {
	return func(c *config) { c.run = info }
}

// runVars returns the run-level built-in variables.
func runVars(info RunInfo, flow, tempDir string) []string {
	if info.Flow == "" {
		info.Flow = flow
	}
	vars := []string{
		"TRUSS_FLOW=" + info.Flow,
		"TRUSS_PIPELINE=" + info.Flow,
		"TRUSS_OS=" + runtime.GOOS,
		"TRUSS_ARCH=" + runtime.GOARCH,
	}
	for _, v := range []struct{ name, value string }{
		{"TRUSS_RUN_ID", info.ID}, {"TRUSS_TRIGGER", info.Trigger},
		{"TRUSS_PROJECT_DIR", info.ProjectDir}, {"TRUSS_TEMP_DIR", tempDir},
	} {
		if v.value != "" {
			vars = append(vars, v.name+"="+v.value)
		}
	}
	return vars
}

// stepVars returns the step-level built-in variables. The step id is the one
// written in the file; TRUSS_JOB is set for a step of a job.
func stepVars(step pipeline.Step, attempt int) []string {
	id := step.ID
	if step.Job != "" {
		id = strings.TrimPrefix(id, step.Job+"/")
	}
	vars := []string{"TRUSS_STEP=" + id, "TRUSS_ATTEMPT=" + strconv.Itoa(attempt)}
	if step.Job != "" {
		vars = append(vars, "TRUSS_JOB="+step.Job)
	}
	return vars
}

// resolveStepEnv builds a step's environment: the run environment, the
// built-in variables, then the step's layers with ${NAME} expanded (spec 026).
// It also returns the step's working directory with ${NAME} expanded.
func resolveStepEnv(base []string, step pipeline.Step, attempt int) ([]string, string, error) {
	env := append(append([]string(nil), base...), stepVars(step, attempt)...)
	lookup := envLookup(env)
	vars, err := pipeline.ExpandLayers(lookup, step.EnvLayers...)
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		env = append(env, name+"="+vars[name])
	}
	// A directory built from a variable that is not set would quietly turn
	// "${INBOX}/x" into "/x"; that fails the step instead (spec 026).
	look := envLookup(env)
	var unset string
	dir, err := pipeline.Expand(step.Dir, func(name string) string {
		v := look(name)
		if v == "" && unset == "" {
			unset = name
		}
		return v
	})
	if err != nil {
		return nil, "", err
	}
	if unset != "" {
		return nil, "", fmt.Errorf("dir uses ${%s}, which is not set", unset)
	}
	return env, dir, nil
}

// removeTempDir removes the run's temporary folder. On Windows a process
// that was just killed can hold a file a little longer, so it tries again
// for up to a second.
func removeTempDir(dir string) {
	for i := 0; ; i++ {
		if err := os.RemoveAll(dir); err == nil || runtime.GOOS != "windows" || i == 9 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// makeTempDir creates the run's temporary folder. A failure leaves
// TRUSS_TEMP_DIR unset rather than stopping the run.
func makeTempDir() string {
	dir, err := os.MkdirTemp("", "truss-run-")
	if err != nil {
		return ""
	}
	return dir
}
