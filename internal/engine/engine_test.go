package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

func TestRun_EmitsStepEventsInOrder(t *testing.T) {
	rec := &eventLog{}
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "hello-world",
		Steps: []pipeline.Step{
			{ID: "hello", Run: echoHello()},
			{ID: "second", Run: "echo second"},
		},
	}
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusSucceeded || len(run.Steps) != 2 {
		t.Fatalf("run = %+v", run)
	}
	phases := rec.phases()
	want := []string{
		"started hello",
		"output hello",
		"finished hello",
		"started second",
		"output second",
		"finished second",
	}
	if strings.Join(phases, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %q", phases)
	}
	hello := rec.output("hello")
	second := rec.output("second")
	if !strings.Contains(hello, "Hello from TRUSS") || !strings.Contains(second, "second") {
		t.Fatalf("output hello=%q second=%q", hello, second)
	}
	for _, chunk := range []string{hello, second} {
		if strings.Contains(chunk, "step ") || strings.Contains(chunk, "[hello]") || strings.Contains(chunk, "[second]") {
			t.Fatalf("engine wrote transcript text: %q", chunk)
		}
	}
	if rec.exit("hello") == nil || *rec.exit("hello") != 0 || rec.exit("second") == nil || *rec.exit("second") != 0 {
		t.Fatalf("exit codes = %+v", rec.events)
	}
}

// eventLog records every event in the order it was received. Concurrent Steps
// report output from their own goroutines, so every method locks.
type eventLog struct {
	mu     sync.Mutex
	events []stepEvent
}

type stepEvent struct {
	kind  string
	id    string
	chunk string
	exit  *int
}

func (r *eventLog) StepStarted(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "started", id: id})
}

func (r *eventLog) StepOutput(id string, chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "output", id: id, chunk: string(chunk)})
}

// StepStopped records that id was ended because failedID failed. The failed
// Step's id is kept in chunk.
func (r *eventLog) StepStopped(id, failedID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "stopped", id: id, chunk: failedID})
}

func (r *eventLog) StepFinished(id string, exitCode *int, startErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ev := stepEvent{kind: "finished", id: id, exit: exitCode}
	if startErr != nil {
		ev.chunk = startErr.Error()
	}
	r.events = append(r.events, ev)
}

func (r *eventLog) StepRetrying(id string, attempt, attempts int, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "retrying", id: id, chunk: fmt.Sprintf("%d/%d %s", attempt, attempts, delay)})
}

func (r *eventLog) StepContinued(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "continued", id: id})
}

// StepSkipped keeps the reason in chunk.
func (r *eventLog) StepSkipped(id, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, stepEvent{kind: "skipped", id: id, chunk: reason})
}

func (r *eventLog) phases() []string {
	var out []string
	var outputID string
	for _, ev := range r.events {
		if ev.kind == "output" {
			if outputID == ev.id {
				continue
			}
			outputID = ev.id
			out = append(out, "output "+ev.id)
			continue
		}
		outputID = ""
		out = append(out, ev.kind+" "+ev.id)
	}
	return out
}

func (r *eventLog) output(id string) string {
	var b strings.Builder
	for _, ev := range r.events {
		if ev.kind == "output" && ev.id == id {
			b.WriteString(ev.chunk)
		}
	}
	return b.String()
}

func (r *eventLog) exit(id string) *int {
	for _, ev := range r.events {
		if ev.kind == "finished" && ev.id == id {
			return ev.exit
		}
	}
	return nil
}

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
	p := pipeline.Pipeline{Version: "2", Name: "n", Steps: []pipeline.Step{{ID: "a", Run: run}}}
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

func TestRun_StepWaitsForTheStepsItNeeds(t *testing.T) {
	rec := &eventLog{}
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "ordered",
		Steps: []pipeline.Step{
			{ID: "test", Run: "echo test", Needs: []string{"build"}},
			{ID: "build", Run: "echo build"},
			{ID: "scan", Run: "echo scan"},
		},
	}
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s, run = %+v", run.Status, run)
	}
	var started []string
	for _, ev := range rec.events {
		if ev.kind == "started" {
			started = append(started, ev.id)
		}
	}
	if strings.Join(started, ",") != "build,test,scan" {
		t.Fatalf("start order = %q", started)
	}
	var ran []string
	for _, step := range run.Steps {
		ran = append(ran, step.ID)
		if step.Status != pipeline.StatusSucceeded || step.ExitCode == nil || *step.ExitCode != 0 {
			t.Fatalf("step = %+v", step)
		}
	}
	if strings.Join(ran, ",") != "build,test,scan" {
		t.Fatalf("result order = %q", ran)
	}
}

func TestRun_FailedStepStartsNothingFurther(t *testing.T) {
	dir := t.TempDir()
	markers := map[string]string{
		"test": filepath.Join(dir, "test.txt"),
		"lint": filepath.Join(dir, "lint.txt"),
	}
	fail := "exit 3"
	if runtime.GOOS == "windows" {
		fail = "exit /b 3"
	}
	write := func(path string) string {
		if runtime.GOOS == "windows" {
			return "echo x> \"" + path + "\""
		}
		return "echo x > '" + path + "'"
	}
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "stops",
		Steps: []pipeline.Step{
			{ID: "test", Run: write(markers["test"]), Needs: []string{"build"}},
			{ID: "build", Run: fail},
			{ID: "lint", Run: write(markers["lint"])},
		},
	}
	run, err := Run(context.Background(), p, dir, nil, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusFailed {
		t.Fatalf("status = %s", run.Status)
	}
	if len(run.Steps) != 1 || run.Steps[0].ID != "build" {
		t.Fatalf("steps = %+v", run.Steps)
	}
	if got := run.Steps[0]; got.Status != pipeline.StatusFailed || got.ExitCode == nil || *got.ExitCode != 3 {
		t.Fatalf("build = %+v", got)
	}
	for id, marker := range markers {
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf("step %s ran after build failed: %v", id, statErr)
		}
	}
}

func TestRun_StartFailureStartsNothingFurther(t *testing.T) {
	p := pipeline.Pipeline{
		Version: "1",
		Name:    "cannot-start",
		Steps: []pipeline.Step{
			{ID: "test", Run: "echo test", Needs: []string{"build"}},
			{ID: "build", Run: "echo build"},
			{ID: "lint", Run: "echo lint"},
		},
	}
	missing := filepath.Join(t.TempDir(), "missing-dir")
	run, err := Run(context.Background(), p, missing, nil, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != pipeline.StatusFailed {
		t.Fatalf("status = %s", run.Status)
	}
	if len(run.Steps) != 1 || run.Steps[0].ID != "build" {
		t.Fatalf("steps = %+v", run.Steps)
	}
	if got := run.Steps[0]; got.Status != pipeline.StatusFailed || got.ExitCode != nil || got.Error == "" {
		t.Fatalf("build = %+v, want a start error and no exit code", got)
	}
}

func echoHello() string {
	if runtime.GOOS == "windows" {
		return "echo Hello from TRUSS"
	}
	return `echo "Hello from TRUSS"`
}

// cancelOnFinish records events and cancels the run's context as soon as the
// first Step has finished by itself.
type cancelOnFinish struct {
	*eventLog
	cancel context.CancelFunc
}

func (c *cancelOnFinish) StepFinished(id string, exitCode *int, startErr error) {
	c.eventLog.StepFinished(id, exitCode, startErr)
	c.cancel()
}

// TestRun_CancelledContextReturnsContextErrorEvenWhenStepExitedByItself covers
// FR-017: once the context is cancelled the run is interrupted, whatever the
// Step's own Exit Code was, and no later Step starts.
func TestRun_CancelledContextReturnsContextErrorEvenWhenStepExitedByItself(t *testing.T) {
	for _, code := range []string{"0", "3"} {
		t.Run("exit "+code, func(t *testing.T) {
			p := pipeline.Pipeline{Version: "1", Name: "interrupted", Steps: []pipeline.Step{
				{ID: "first", Run: helperCmd("exit", code)},
				{ID: "second", Run: helperCmd("exit", "0")},
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			events := &cancelOnFinish{eventLog: &eventLog{}, cancel: cancel}

			run, err := Run(ctx, p, t.TempDir(), nil, events)

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled; run = %+v", err, run)
			}
			if run.Status == pipeline.StatusFailed {
				t.Fatalf("interrupted run reported as failed: %+v", run)
			}
			for _, ev := range events.events {
				if ev.id == "second" {
					t.Fatalf("later Step produced event %+v", ev)
				}
			}
		})
	}
}
