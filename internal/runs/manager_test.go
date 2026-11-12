package runs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/pipeline"
)

// The test binary doubles as a step command: "-truss-helper wait <file>"
// blocks until the file exists, so tests decide when a step ends.
const helperFlag = "-truss-helper"

func TestMain(m *testing.M) {
	if len(os.Args) > 3 && os.Args[1] == helperFlag && os.Args[2] == "wait" {
		for {
			if _, err := os.Stat(os.Args[3]); err == nil {
				os.Exit(0)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	_ = os.Setenv("GORACE", "atexit_sleep_ms=0")
	os.Exit(m.Run())
}

func quote(s string) string {
	if runtime.GOOS == "windows" && !strings.ContainsAny(s, " \t") {
		return s
	}
	return `"` + s + `"`
}

// waitFor returns a run string that blocks until gate exists.
func waitFor(gate string) string {
	exe, _ := os.Executable()
	return quote(exe) + " " + helperFlag + " wait " + quote(gate)
}

func open(t *testing.T, opts Options) *Manager {
	t.Helper()
	if opts.Dir == "" {
		opts.Dir = t.TempDir()
	}
	opts.Env = os.Environ()
	m, err := Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func def(name string, overlap pipeline.Overlap, steps ...pipeline.Step) pipeline.Pipeline {
	return pipeline.Pipeline{Version: "1", Name: name, Overlap: overlap, Steps: steps}
}

func submit(t *testing.T, m *Manager, name string, d pipeline.Pipeline) api.Run {
	t.Helper()
	run, err := m.Submit(Request{Pipeline: name, Definition: d, Dir: t.TempDir(), Trigger: api.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// await polls until the run reaches one of the statuses.
func await(t *testing.T, m *Manager, id string, statuses ...string) api.Run {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		run, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range statuses {
			if run.Status == s {
				return run
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s is %s, want %v", id, run.Status, statuses)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// awaitStep polls until the step of the run has the given status. A run is
// "running" slightly before its first step starts, so tests that act on a
// step wait for the step itself.
func awaitStep(t *testing.T, m *Manager, id, step, status string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		run, err := m.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range run.Steps {
			if s.ID == step && s.Status == status {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("step %s of run %s never became %s: %+v", step, id, status, run.Steps)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func release(t *testing.T, gate string) {
	t.Helper()
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSubmit_RunsAndStoresLogAndSteps(t *testing.T) {
	m := open(t, Options{})
	run := submit(t, m, "hello", def("hello", "",
		pipeline.Step{ID: "one", Run: "echo first"},
		pipeline.Step{ID: "two", Run: "echo second", Needs: []string{"one"}},
	))
	done := await(t, m, run.ID, api.RunSucceeded, api.RunFailed)
	if done.Status != api.RunSucceeded || done.StartedAt == nil || done.FinishedAt == nil {
		t.Fatalf("run = %+v", done)
	}
	for _, s := range done.Steps {
		if s.Status != api.StepSucceeded || s.ExitCode == nil || *s.ExitCode != 0 {
			t.Fatalf("step = %+v", s)
		}
	}
	data, next, status, err := m.Log(run.ID, 0)
	if err != nil || status != api.RunSucceeded || next != int64(len(data)) {
		t.Fatalf("log: %v %s %d", err, status, next)
	}
	text := string(data)
	for _, want := range []string{"setup environment ready", "step one | first", "step two | second", "summary succeeded steps 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("log lacks %q:\n%s", want, text)
		}
	}
	if rest, _, _, _ := m.Log(run.ID, next); len(rest) != 0 {
		t.Fatalf("log continued: %q", rest)
	}
}

func TestSubmit_FailedStepFailsTheRunAndSkipsTheRest(t *testing.T) {
	m := open(t, Options{})
	run := submit(t, m, "bad", def("bad", "",
		pipeline.Step{ID: "boom", Run: "exit 3"},
		pipeline.Step{ID: "after", Run: "echo never", Needs: []string{"boom"}},
	))
	done := await(t, m, run.ID, api.RunSucceeded, api.RunFailed)
	if done.Status != api.RunFailed || done.Reason != "bad failed at boom (exit 3)" {
		t.Fatalf("run = %+v", done)
	}
	if done.Steps[0].Status != api.StepFailed || *done.Steps[0].ExitCode != 3 || done.Steps[1].Status != api.StepSkipped {
		t.Fatalf("steps = %+v", done.Steps)
	}
}

func TestSubmit_StepsSeeTheRunVariables(t *testing.T) {
	m := open(t, Options{})
	show := "echo $TRUSS_PIPELINE $TRUSS_TRIGGER $TRUSS_RUN_ID"
	if runtime.GOOS == "windows" {
		show = "echo %TRUSS_PIPELINE% %TRUSS_TRIGGER% %TRUSS_RUN_ID%"
	}
	run := submit(t, m, "vars", def("vars", "", pipeline.Step{ID: "show", Run: show}))
	await(t, m, run.ID, api.RunSucceeded)
	data, _, _, _ := m.Log(run.ID, 0)
	if want := "step show | vars manual " + run.ID; !strings.Contains(string(data), want) {
		t.Fatalf("log lacks %q:\n%s", want, data)
	}
}

func TestOverlap_QueueRunsOneAtATime(t *testing.T) {
	m := open(t, Options{})
	gate := filepath.Join(t.TempDir(), "gate")
	d := def("p", pipeline.OverlapQueue, pipeline.Step{ID: "wait", Run: waitFor(gate)})
	first := submit(t, m, "p", d)
	second := submit(t, m, "p", d)
	await(t, m, first.ID, api.RunRunning)
	if got, _ := m.Get(second.ID); got.Status != api.RunQueued {
		t.Fatalf("second run is %s", got.Status)
	}
	if running, queued := m.Counts(); running != 1 || queued != 1 {
		t.Fatalf("counts = %d running, %d queued", running, queued)
	}
	release(t, gate)
	await(t, m, first.ID, api.RunSucceeded)
	await(t, m, second.ID, api.RunSucceeded)
}

func TestOverlap_SkipRecordsTheSecondRequest(t *testing.T) {
	m := open(t, Options{})
	gate := filepath.Join(t.TempDir(), "gate")
	d := def("p", pipeline.OverlapSkip, pipeline.Step{ID: "wait", Run: waitFor(gate)})
	first := submit(t, m, "p", d)
	second := submit(t, m, "p", d)
	if second.Status != api.RunSkipped || second.Reason == "" || second.Steps[0].Status != api.StepSkipped {
		t.Fatalf("second = %+v", second)
	}
	release(t, gate)
	await(t, m, first.ID, api.RunSucceeded)
}

func TestOverlap_ParallelRunsTogether(t *testing.T) {
	m := open(t, Options{})
	gate := filepath.Join(t.TempDir(), "gate")
	d := def("p", pipeline.OverlapParallel, pipeline.Step{ID: "wait", Run: waitFor(gate)})
	first, second := submit(t, m, "p", d), submit(t, m, "p", d)
	await(t, m, first.ID, api.RunRunning)
	await(t, m, second.ID, api.RunRunning)
	release(t, gate)
	await(t, m, first.ID, api.RunSucceeded)
	await(t, m, second.ID, api.RunSucceeded)
}

func TestMaxParallel_LimitsRunsAcrossPipelines(t *testing.T) {
	m := open(t, Options{MaxParallel: 1})
	gate := filepath.Join(t.TempDir(), "gate")
	a := submit(t, m, "a", def("a", "", pipeline.Step{ID: "wait", Run: waitFor(gate)}))
	b := submit(t, m, "b", def("b", "", pipeline.Step{ID: "hi", Run: "echo hi"}))
	await(t, m, a.ID, api.RunRunning)
	if got, _ := m.Get(b.ID); got.Status != api.RunQueued {
		t.Fatalf("b is %s while the only slot is taken", got.Status)
	}
	release(t, gate)
	await(t, m, b.ID, api.RunSucceeded)
}

func TestCancel_EndsARunningRunAndAQueuedRun(t *testing.T) {
	m := open(t, Options{})
	gate := filepath.Join(t.TempDir(), "never")
	d := def("p", "", pipeline.Step{ID: "wait", Run: waitFor(gate)}, pipeline.Step{ID: "after", Run: "echo x", Needs: []string{"wait"}})
	first, second := submit(t, m, "p", d), submit(t, m, "p", d)
	await(t, m, first.ID, api.RunRunning)
	awaitStep(t, m, first.ID, "wait", api.StepRunning)

	if got, err := m.Cancel(second.ID); err != nil || got.Status != api.RunCancelled {
		t.Fatalf("cancel queued: %+v %v", got, err)
	}
	if _, err := m.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	done := await(t, m, first.ID, api.RunCancelled, api.RunFailed, api.RunSucceeded)
	if done.Status != api.RunCancelled || done.Steps[0].Status != api.StepCancelled || done.Steps[1].Status != api.StepSkipped {
		t.Fatalf("run = %+v", done)
	}
	data, _, _, _ := m.Log(first.ID, 0)
	if !strings.Contains(string(data), "summary cancelled steps 1") {
		t.Fatalf("log:\n%s", data)
	}
	if _, err := m.Cancel("nope"); err == nil {
		t.Fatal("unknown id accepted")
	}
}

func TestRecordAndHistorySurviveReopen(t *testing.T) {
	dir := t.TempDir()
	m := open(t, Options{Dir: dir})
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	skipped, err := m.Record("nightly", api.TriggerSchedule, at, api.RunSkipped, "missed")
	if err != nil {
		t.Fatal(err)
	}
	ran := submit(t, m, "nightly", def("nightly", "", pipeline.Step{ID: "hi", Run: "echo hi"}))
	await(t, m, ran.ID, api.RunSucceeded)
	m.Close()

	again := open(t, Options{Dir: dir})
	list := again.List("nightly", 0)
	if len(list) != 2 || list[0].ID != ran.ID || list[1].ID != skipped.ID {
		t.Fatalf("history = %+v", list)
	}
	if list[1].ScheduledFor == nil || !list[1].ScheduledFor.Equal(at) || list[1].Reason != "missed" {
		t.Fatalf("skipped = %+v", list[1])
	}
	if active, last := again.Summary("nightly"); active != 0 || last == nil || last.ID != ran.ID {
		t.Fatalf("summary = %d %+v", active, last)
	}
	if len(again.List("other", 0)) != 0 || len(again.List("", 1)) != 1 {
		t.Fatal("filter or limit ignored")
	}
}

func TestOpen_ClosesRunsLeftActiveByAStoppedDaemon(t *testing.T) {
	dir := t.TempDir()
	id := "20261111-090000-abcdef"
	if err := os.MkdirAll(filepath.Join(dir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"id":%q,"pipeline":"p","trigger":"manual","status":"running","created_at":"2026-11-11T09:00:00Z","steps":[{"id":"a","status":"running"},{"id":"b","status":"pending"}]}`, id)
	if err := os.WriteFile(filepath.Join(dir, id, "run.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := open(t, Options{Dir: dir})
	run, err := m.Get(id)
	if err != nil || run.Status != api.RunFailed || run.Steps[0].Status != api.StepCancelled || run.Steps[1].Status != api.StepSkipped {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
}

func TestHistory_KeepsOnlyTheNewestRuns(t *testing.T) {
	m := open(t, Options{Keep: 3})
	var ids []string
	for i := range 5 {
		run, err := m.Record("p", api.TriggerSchedule, time.Time{}, api.RunSkipped, fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, run.ID)
	}
	list := m.List("", 0)
	if len(list) != 3 || list[0].Reason != "4" || list[2].Reason != "2" {
		t.Fatalf("history = %+v", list)
	}
	if _, err := os.Stat(filepath.Join(m.opts.Dir, ids[0])); !os.IsNotExist(err) {
		t.Fatalf("pruned run still on disk: %v", err)
	}
}

// Runs created in the same clock instant (a coarse clock, or a missed schedule
// recorded in the same tick as a new run) keep their creation order after the
// daemon restarts.
func TestHistory_KeepsCreationOrderWhenTheClockDoesNotMove(t *testing.T) {
	dir := t.TempDir()
	frozen := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	m := open(t, Options{Dir: dir, Now: func() time.Time { return frozen }})
	for i := range 8 {
		if _, err := m.Record("p", api.TriggerSchedule, time.Time{}, api.RunSkipped, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	m.Close()

	again := open(t, Options{Dir: dir})
	list := again.List("", 0)
	if len(list) != 8 {
		t.Fatalf("history has %d runs", len(list))
	}
	for i, run := range list {
		if want := fmt.Sprint(7 - i); run.Reason != want {
			t.Fatalf("position %d is run %s, want run %s (newest first)", i, run.Reason, want)
		}
	}
}

// resolveTo returns a Resolver that serves definitions by pipeline name.
func resolveTo(dir string, defs map[string]pipeline.Pipeline) Resolver {
	return func(name string) (pipeline.Pipeline, string, error) {
		d, ok := defs[name]
		if !ok {
			return pipeline.Pipeline{}, "", fmt.Errorf("%w: %q", ErrNotFound, name)
		}
		return d, dir, nil
	}
}

func TestQueue_SurvivesAGracefulRestartAndStartsInRequestOrder(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(t.TempDir(), "gate")
	d := def("p", pipeline.OverlapQueue, pipeline.Step{ID: "wait", Run: waitFor(gate)})
	m := open(t, Options{Dir: dir})
	running := submit(t, m, "p", d)
	second, third := submit(t, m, "p", d), submit(t, m, "p", d)
	await(t, m, running.ID, api.RunRunning)
	m.Close() // a graceful stop: the running run ends, the queue stays

	if got, _ := m.Get(second.ID); got.Status != api.RunQueued {
		t.Fatalf("queued run after Close = %s", got.Status)
	}
	release(t, gate)
	again := open(t, Options{Dir: dir, Resolve: resolveTo(t.TempDir(), map[string]pipeline.Pipeline{"p": d})})
	if got, _ := again.Get(running.ID); got.Status != api.RunCancelled {
		t.Fatalf("run that was running = %s", got.Status)
	}
	if running, queued := again.Counts(); running != 0 || queued != 2 {
		t.Fatalf("before Resume: %d running, %d queued", running, queued)
	}
	again.Resume()
	a := await(t, again, second.ID, api.RunSucceeded, api.RunFailed)
	b := await(t, again, third.ID, api.RunSucceeded, api.RunFailed)
	if a.Status != api.RunSucceeded || b.Status != api.RunSucceeded {
		t.Fatalf("restored runs: %s, %s", a.Status, b.Status)
	}
	if a.StartedAt.After(*b.StartedAt) || a.FinishedAt.After(*b.StartedAt) {
		t.Fatalf("overlap queue started out of order: %v..%v then %v", a.StartedAt, a.FinishedAt, b.StartedAt)
	}
}

func TestQueue_RestoredRunsKeepTheirTriggerAndRespectLimits(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(t.TempDir(), "gate")
	d := def("p", pipeline.OverlapParallel, pipeline.Step{ID: "wait", Run: waitFor(gate)})
	first := open(t, Options{Dir: dir, MaxParallel: 1})
	r1 := submit(t, first, "p", d)
	r2, err := first.Submit(Request{Pipeline: "p", Definition: d, Dir: t.TempDir(), Trigger: api.TriggerSchedule, ScheduledFor: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	await(t, first, r1.ID, api.RunRunning)
	first.Close()

	again := open(t, Options{Dir: dir, MaxParallel: 1, Resolve: resolveTo(t.TempDir(), map[string]pipeline.Pipeline{"p": d})})
	again.Resume()
	got := await(t, again, r2.ID, api.RunRunning)
	if got.Trigger != api.TriggerSchedule || got.ScheduledFor == nil {
		t.Fatalf("restored run lost its origin: %+v", got)
	}
	release(t, gate)
	await(t, again, r2.ID, api.RunSucceeded)
}

func TestQueue_RunThatCannotBeRestoredFailsWithTheReason(t *testing.T) {
	for name, resolve := range map[string]Resolver{
		"no resolver":      nil,
		"pipeline is gone": resolveTo(t.TempDir(), nil),
		"definition invalid": func(string) (pipeline.Pipeline, string, error) {
			return pipeline.Pipeline{}, "", errors.New("step wait: run is empty")
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			gate := filepath.Join(t.TempDir(), "never")
			d := def("p", pipeline.OverlapQueue, pipeline.Step{ID: "wait", Run: waitFor(gate)})
			m := open(t, Options{Dir: dir})
			submit(t, m, "p", d)
			queued := submit(t, m, "p", d)
			m.Close()

			again := open(t, Options{Dir: dir, Resolve: resolve})
			again.Resume()
			got, err := again.Get(queued.ID)
			if err != nil || got.Status != api.RunFailed || !strings.Contains(got.Reason, "could not be restored") {
				t.Fatalf("run = %+v, err = %v", got, err)
			}
			if got.Steps[0].Status != api.StepSkipped {
				t.Fatalf("steps = %+v", got.Steps)
			}
		})
	}
}

func TestQueue_CancelWorksOnARestoredQueuedRun(t *testing.T) {
	dir := t.TempDir()
	d := def("p", pipeline.OverlapQueue, pipeline.Step{ID: "wait", Run: waitFor(filepath.Join(t.TempDir(), "never"))})
	m := open(t, Options{Dir: dir})
	submit(t, m, "p", d)
	queued := submit(t, m, "p", d)
	m.Close()

	again := open(t, Options{Dir: dir, Resolve: resolveTo(t.TempDir(), map[string]pipeline.Pipeline{"p": d})})
	if got, err := again.Cancel(queued.ID); err != nil || got.Status != api.RunCancelled {
		t.Fatalf("cancel = %+v, %v", got, err)
	}
	again.Resume()
	if _, queuedNow := again.Counts(); queuedNow != 0 {
		t.Fatalf("cancelled run still queued")
	}
}
