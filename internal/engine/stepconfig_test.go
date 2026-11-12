package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

// echoVar returns a run string that prints the value of an environment variable.
func echoVar(name string) string {
	if runtime.GOOS == "windows" {
		return "echo %" + name + "%"
	}
	return "echo $" + name
}

func printDir() string {
	if runtime.GOOS == "windows" {
		return "cd"
	}
	return "pwd"
}

func TestRun_StepEnvOverridesTheRunEnvironment(t *testing.T) {
	p := pipeline.Pipeline{
		Version: "1", Name: "env",
		Env: map[string]string{"TRUSS_A": "pipeline", "TRUSS_B": "pipeline"},
		Steps: []pipeline.Step{
			{ID: "one", Run: echoVar("TRUSS_A") + " && " + echoVar("TRUSS_B") + " && " + echoVar("TRUSS_C"),
				Env: map[string]string{"TRUSS_B": "step"}},
		},
	}
	base := append(os.Environ(), "TRUSS_A=invoker", "TRUSS_C=invoker")
	run, err := Run(context.Background(), p, t.TempDir(), base, nil)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if got := strings.Fields(run.Steps[0].Output); strings.Join(got, " ") != "pipeline step invoker" {
		t.Fatalf("output = %q", run.Steps[0].Output)
	}
}

func TestRun_StepDirIsResolvedAgainstTheRunDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	p := pipeline.Pipeline{
		Version: "1", Name: "dir",
		Steps: []pipeline.Step{{ID: "where", Run: printDir(), Dir: "sub"}},
	}
	run, err := Run(context.Background(), p, dir, os.Environ(), nil)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if got := strings.TrimSpace(run.Steps[0].Output); filepath.Base(got) != "sub" {
		t.Fatalf("step ran in %q", got)
	}
}

func TestRun_MissingStepDirFailsTheStepWithoutAnExitCode(t *testing.T) {
	p := pipeline.Pipeline{
		Version: "1", Name: "dir",
		Steps: []pipeline.Step{{ID: "where", Run: printDir(), Dir: "nope"}},
	}
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != pipeline.StatusFailed || run.Steps[0].ExitCode != nil || run.Steps[0].Error == "" {
		t.Fatalf("run = %+v", run)
	}
}

func TestRun_StepTimeoutFailsTheStepAndTheRun(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	p := pipeline.Pipeline{
		Version: "1", Name: "slow",
		Steps: []pipeline.Step{
			{ID: "stuck", Run: helperCmd("block", pidFile), Timeout: 300 * time.Millisecond},
			{ID: "after", Run: "echo never", Needs: []string{"stuck"}},
		},
	}
	start := time.Now()
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 20*time.Second {
		t.Fatalf("timeout did not end the step: %s", time.Since(start))
	}
	if run.Status != pipeline.StatusFailed || run.FailedStep != "stuck" || len(run.Steps) != 1 {
		t.Fatalf("run = %+v", run)
	}
	step := run.Steps[0]
	if step.ExitCode != nil || step.Error != "timed out after 300ms" || step.StoppedBy != "" {
		t.Fatalf("step = %+v", step)
	}
}

func TestRun_JobsRunInDependencyOrder(t *testing.T) {
	p := pipeline.Pipeline{
		Version: "1", Name: "jobs", Concurrency: 4,
		Jobs: []pipeline.Job{
			{ID: "deploy", Needs: []string{"build"}, Steps: []pipeline.Step{{ID: "ship", Run: "echo ship"}}},
			{ID: "build", Steps: []pipeline.Step{{ID: "a", Run: "echo a"}, {ID: "b", Run: "echo b"}}},
		},
	}
	rec := &eventLog{}
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), rec)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	var started []string
	for _, phase := range rec.phases() {
		if id, ok := strings.CutPrefix(phase, "started "); ok {
			started = append(started, id)
		}
	}
	if strings.Join(started, " ") != "build/a build/b deploy/ship" {
		t.Fatalf("start order = %v", started)
	}
}
