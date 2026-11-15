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

func TestRun_WorkdirPrecedenceStepThenJobThenWorkdirThenRunDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"base", "jobdir", "stepdir"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	p, err := pipeline.Parse([]byte(`
version: "1"
pipeline:
  name: dirs
  workdir: base
jobs:
  a:
    dir: jobdir
    steps:
      - id: own
        run: ` + printDir() + `
        dir: stepdir
      - id: inherit
        run: ` + printDir() + `
  b:
    steps:
      - id: plain
        run: ` + printDir() + `
`))
	if err != nil {
		t.Fatal(err)
	}
	run, err := Run(context.Background(), p, dir, os.Environ(), nil)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	want := map[string]string{"a/own": "stepdir", "a/inherit": "jobdir", "b/plain": "base"}
	for _, s := range run.Steps {
		if got := filepath.Base(strings.TrimSpace(s.Output)); got != want[s.ID] {
			t.Errorf("%s ran in %q, want %q", s.ID, got, want[s.ID])
		}
	}

	// Without a workdir, steps start in the run directory.
	flat := pipeline.Pipeline{Version: "1", Name: "flat", Steps: []pipeline.Step{{ID: "s", Run: printDir()}}}
	run, err = Run(context.Background(), flat, dir, os.Environ(), nil)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("flat run = %+v, err = %v", run, err)
	}
	if got := filepath.Base(strings.TrimSpace(run.Steps[0].Output)); got != filepath.Base(dir) {
		t.Fatalf("flat step ran in %q", got)
	}
}

func TestRun_AbsoluteWorkdirIsUsedAsIs(t *testing.T) {
	target := t.TempDir()
	p := pipeline.Pipeline{Version: "1", Name: "abs", Workdir: target, Steps: []pipeline.Step{{ID: "s", Run: printDir()}}}
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil || run.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if got := filepath.Base(strings.TrimSpace(run.Steps[0].Output)); got != filepath.Base(target) {
		t.Fatalf("step ran in %q, want %q", got, target)
	}
}

func TestRun_MissingWorkdirFailsTheStepWithoutAnExitCode(t *testing.T) {
	p := pipeline.Pipeline{Version: "1", Name: "gone", Workdir: "nope", Steps: []pipeline.Step{
		{ID: "first", Run: printDir()},
		{ID: "second", Run: printDir(), Needs: []string{"first"}},
	}}
	run, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != pipeline.StatusFailed || len(run.Steps) != 1 || run.Steps[0].ExitCode != nil || run.Steps[0].Error == "" {
		t.Fatalf("run = %+v", run)
	}
}

func TestStepDir(t *testing.T) {
	sep := string(filepath.Separator)
	abs := filepath.Join(t.TempDir(), "abs")
	cases := []struct {
		name, run, dir, want string
		windows, wantErr     bool
	}{
		{name: "empty", run: "run", dir: "", want: "run"},
		{name: "relative", run: "run", dir: "sub", want: "run" + sep + "sub"},
		{name: "dot dot", run: "run" + sep + "x", dir: "..", want: "run"},
		{name: "absolute", run: "run", dir: abs, want: abs},
		{name: "drive absolute", run: `C:\run`, dir: `D:\work`, want: `D:\work`, windows: true},
		{name: "drive absolute slash", run: `C:\run`, dir: `D:/work`, want: `D:\work`, windows: true},
		{name: "rooted takes the run drive", run: `C:\run`, dir: `\work`, want: `C:\work`, windows: true},
		{name: "rooted slash", run: `C:\run`, dir: `/work`, want: `C:\work`, windows: true},
		{name: "drive relative is refused", run: `C:\run`, dir: `D:work`, windows: true, wantErr: true},
		{name: "relative on a drive", run: `C:\run`, dir: `sub\x`, want: `C:\run\sub\x`, windows: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.windows && runtime.GOOS != "windows" {
				t.Skip("Windows path rules")
			}
			got, err := stepDir(c.run, c.dir)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if err == nil && filepath.Clean(got) != filepath.Clean(c.want) {
				t.Fatalf("stepDir(%q, %q) = %q, want %q", c.run, c.dir, got, c.want)
			}
		})
	}
}
