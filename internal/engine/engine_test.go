package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/afri-bit/truss/internal/pipeline"
)

func TestRun_TwoSuccessfulSteps(t *testing.T) {
	second := "echo second"
	if runtime.GOOS == "windows" {
		second = "echo second"
	}
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "hello-world",
		Steps: []pipeline.Step{
			{ID: "hello", Run: echoHello()},
			{ID: "second", Run: second},
		},
	}
	run, err := Run(context.Background(), p, t.TempDir(), nil, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s", run.Status)
	}
	if len(run.Steps) != 2 || run.Steps[0].ID != "hello" || run.Steps[1].ID != "second" {
		t.Fatalf("steps = %+v", run.Steps)
	}
	for _, step := range run.Steps {
		if step.Status != pipeline.StatusSucceeded || step.ExitCode == nil || *step.ExitCode != 0 {
			t.Fatalf("step = %+v", step)
		}
	}
}

func TestRun_InvalidModelDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran.txt")
	run := "echo x > '" + marker + "'"
	if runtime.GOOS == "windows" {
		run = "echo x> \"" + marker + "\""
	}
	p := pipeline.Pipeline{Version: "1", Steps: []pipeline.Step{{ID: "a", Run: run}}}
	_, err := Run(context.Background(), p, dir, nil, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("command ran: %v", statErr)
	}
}

func TestRun_StopsAfterFailedStep(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "third.txt")
	fail := "exit 1"
	third := "echo third > '" + marker + "'"
	if runtime.GOOS == "windows" {
		fail = "exit /b 1"
		third = "echo third> \"" + marker + "\""
	}
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "fail-fast",
		Steps: []pipeline.Step{
			{ID: "first", Run: echoHello()},
			{ID: "second", Run: fail},
			{ID: "third", Run: third},
		},
	}
	run, err := Run(context.Background(), p, dir, nil, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusFailed {
		t.Fatalf("status = %s", run.Status)
	}
	if len(run.Steps) != 2 || run.Steps[1].ID != "second" || run.Steps[1].ExitCode == nil || *run.Steps[1].ExitCode == 0 {
		t.Fatalf("steps = %+v", run.Steps)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("third step ran: %v", statErr)
	}
}

func echoHello() string {
	if runtime.GOOS == "windows" {
		return "echo Hello from TRUSS"
	}
	return `echo "Hello from TRUSS"`
}
