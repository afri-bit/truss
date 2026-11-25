package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/pipeline"
)

// Spec 026: every step sees the built-in variables.
func TestRun_StepsSeeTheBuiltInVariables(t *testing.T) {
	vars := []string{"TRUSS_FLOW", "TRUSS_PIPELINE", "TRUSS_STEP", "TRUSS_JOB", "TRUSS_ATTEMPT",
		"TRUSS_RUN_ID", "TRUSS_TRIGGER", "TRUSS_PROJECT_DIR", "TRUSS_OS", "TRUSS_ARCH"}
	run := make([]string, len(vars))
	for i, v := range vars {
		run[i] = echoVar(v)
	}
	p := pipeline.Pipeline{Version: "1", Name: "built", Jobs: []pipeline.Job{
		{ID: "build", Steps: []pipeline.Step{{ID: "show", Run: strings.Join(run, " && ")}}},
	}}
	base := append(os.Environ(), "TRUSS_FLOW=from-the-shell")
	got, err := Run(context.Background(), p, t.TempDir(), base, nil,
		WithRun(RunInfo{ID: "r1", Trigger: "manual", ProjectDir: "/proj"}))
	if err != nil || got.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", got, err)
	}
	want := []string{"built", "built", "show", "build", "1", "r1", "manual", "/proj", runtime.GOOS, runtime.GOARCH}
	if fields := strings.Fields(got.Steps[0].Output); strings.Join(fields, " ") != strings.Join(want, " ") {
		t.Fatalf("output = %q, want %q", got.Steps[0].Output, want)
	}
}

func TestRun_TempDirIsMadeForTheRunAndRemovedAfter(t *testing.T) {
	marker := "made.txt"
	write := "echo x> \"$TRUSS_TEMP_DIR/" + marker + "\" && echo $TRUSS_TEMP_DIR"
	if runtime.GOOS == "windows" {
		write = "echo x> \"%TRUSS_TEMP_DIR%\\" + marker + "\" && echo %TRUSS_TEMP_DIR%"
	}
	p := pipeline.Pipeline{Version: "1", Name: "tmp", Steps: []pipeline.Step{{ID: "a", Run: write}}}
	got, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil || got.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", got, err)
	}
	dir := strings.TrimSpace(got.Steps[0].Output)
	if dir == "" || !filepath.IsAbs(dir) {
		t.Fatalf("TRUSS_TEMP_DIR = %q", dir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir %s still there after the run: %v", dir, err)
	}
}

func TestRun_ExpandsVariablesInEnvAndDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "work", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := pipeline.Pipeline{
		Version: "1", Name: "x",
		ProjectEnv: map[string]string{"BASE": "${ROOT_FROM_HOST}/work"},
		Env:        map[string]string{"APP": "${BASE}/app", "PRICE": "5$$"},
		Steps: []pipeline.Step{{ID: "a", Run: echoVar("APP") + " && " + echoVar("PRICE") + " && " + printDir(),
			Dir: "${APP}"}},
	}
	got, err := Run(context.Background(), p, root, append(os.Environ(), "ROOT_FROM_HOST="+root), nil)
	if err != nil || got.Status != pipeline.StatusSucceeded {
		t.Fatalf("run = %+v, err = %v", got, err)
	}
	app := filepath.Join(root, "work", "app")
	lines := strings.Fields(got.Steps[0].Output)
	if len(lines) != 3 || filepath.Clean(lines[0]) != app || lines[1] != "5$" {
		t.Fatalf("output = %q", got.Steps[0].Output)
	}
	if real, _ := filepath.EvalSymlinks(app); filepath.Clean(lines[2]) != app && filepath.Clean(lines[2]) != real {
		t.Fatalf("dir = %q, want %q", lines[2], app)
	}
}

func TestRun_AnExpansionLoopFailsTheStepWithoutStartingIt(t *testing.T) {
	p := pipeline.Pipeline{Version: "1", Name: "loop",
		Env:   map[string]string{"A": "${B}", "B": "${A}"},
		Steps: []pipeline.Step{{ID: "a", Run: "echo never"}}}
	got, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != pipeline.StatusFailed || got.Steps[0].ExitCode != nil || !strings.Contains(got.Steps[0].Error, "loop") {
		t.Fatalf("run = %+v", got)
	}
}
