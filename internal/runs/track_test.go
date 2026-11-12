package runs

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/pipeline"
)

func graph(ids ...string) []api.PlanStep {
	var steps []api.PlanStep
	for _, id := range ids {
		steps = append(steps, api.PlanStep{ID: id})
	}
	return steps
}

func exit(code int) *int { return &code }

func TestTrack_RecordsStepsLogAndOutcome(t *testing.T) {
	m := open(t, Options{})
	run, err := m.Track("local-build", graph("a", "b"))
	if err != nil || run.Status != api.RunRunning || run.Trigger != api.TriggerLocal || run.StartedAt == nil || len(run.Steps) != 2 {
		t.Fatalf("run = %+v, err = %v", run, err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepRunning}); err != nil {
		t.Fatal(err)
	}
	got, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepSucceeded, ExitCode: exit(0)})
	if err != nil || got.Steps[0].Status != api.StepSucceeded || got.Steps[0].StartedAt == nil || got.Steps[0].FinishedAt == nil {
		t.Fatalf("step = %+v, err = %v", got.Steps[0], err)
	}
	next, status, err := m.TrackLog(run.ID, 0, []byte("one\n"))
	if err != nil || next != 4 || status != api.RunRunning {
		t.Fatalf("append: %d %s %v", next, status, err)
	}
	if next, _, err = m.TrackLog(run.ID, 4, []byte("two\n")); err != nil || next != 8 {
		t.Fatalf("append: %d %v", next, err)
	}
	// An empty append only reads the status.
	if next, _, err = m.TrackLog(run.ID, 8, nil); err != nil || next != 8 {
		t.Fatalf("empty append: %d %v", next, err)
	}
	data, _, _, err := m.Log(run.ID, 0)
	if err != nil || string(data) != "one\ntwo\n" {
		t.Fatalf("log = %q, %v", data, err)
	}

	done, err := m.TrackFinish(run.ID, api.RunFailed, "local-build failed at b (exit 2)")
	if err != nil || done.Status != api.RunFailed || done.FinishedAt == nil {
		t.Fatalf("finish = %+v, %v", done, err)
	}
	if done.Steps[1].Status != api.StepSkipped {
		t.Fatalf("an unreported step is %s, want skipped", done.Steps[1].Status)
	}
	if _, err := m.TrackFinish(run.ID, api.RunSucceeded, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("second finish: %v", err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "b", Status: api.StepRunning}); !errors.Is(err, ErrConflict) {
		t.Fatalf("step after finish: %v", err)
	}
	if _, _, err := m.TrackLog(run.ID, 8, []byte("late")); !errors.Is(err, ErrConflict) {
		t.Fatalf("log after finish: %v", err)
	}
}

func TestTrack_RejectsBadRequests(t *testing.T) {
	m := open(t, Options{})
	long := strings.Repeat("x", api.MaxTrackedName+1)
	for name, c := range map[string]struct {
		pipeline string
		steps    []api.PlanStep
	}{
		"no name":         {"", graph("a")},
		"long name":       {long, graph("a")},
		"control in name": {"a\nb", graph("a")},
		"no steps":        {"p", nil},
		"empty step id":   {"p", graph("")},
		"repeated step":   {"p", graph("a", "a")},
		"too many steps":  {"p", make([]api.PlanStep, api.MaxTrackedSteps+1)},
	} {
		if _, err := m.Track(c.pipeline, c.steps); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	run, err := m.Track("p", graph("a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "nope", Status: api.StepRunning}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown step: %v", err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: "pending"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad step status: %v", err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepFailed, Error: strings.Repeat("e", api.MaxReason+1)}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("long error: %v", err)
	}
	if _, err := m.TrackFinish(run.ID, "running", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad run status: %v", err)
	}
	if _, _, err := m.TrackLog(run.ID, 0, make([]byte, api.MaxLogChunk+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big append: %v", err)
	}
	// A step that ended cannot change.
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepSucceeded}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepFailed}); !errors.Is(err, ErrConflict) {
		t.Errorf("step changed after it ended: %v", err)
	}
	for _, id := range []string{"nope", "../x", ""} {
		if _, err := m.TrackFinish(id, api.RunSucceeded, ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("finish %q: %v", id, err)
		}
	}
}

func TestTrack_LogAppendsMustContinueTheLog(t *testing.T) {
	m := open(t, Options{})
	run, _ := m.Track("p", graph("a"))
	if _, _, err := m.TrackLog(run.ID, 5, []byte("gap")); !errors.Is(err, ErrConflict) {
		t.Fatalf("append past the end: %v", err)
	}
	if _, _, err := m.TrackLog(run.ID, -1, []byte("x")); !errors.Is(err, ErrConflict) {
		t.Fatalf("negative offset: %v", err)
	}
	if _, _, err := m.TrackLog(run.ID, 0, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	// A retry after a lost answer changes nothing.
	if next, _, err := m.TrackLog(run.ID, 0, []byte("hello\n")); err != nil || next != 6 {
		t.Fatalf("retry: %d %v", next, err)
	}
	// Overlapping the end is not a retry.
	if _, _, err := m.TrackLog(run.ID, 3, []byte("lo\nmore\n")); !errors.Is(err, ErrConflict) {
		t.Fatalf("overlapping append: %v", err)
	}
	if data, _, _, _ := m.Log(run.ID, 0); string(data) != "hello\n" {
		t.Fatalf("log = %q", data)
	}
}

func TestTrack_OnlyTrackedRunsAcceptReports(t *testing.T) {
	m := open(t, Options{})
	ran := submit(t, m, "p", def("p", "", pipeline.Step{ID: "a", Run: "echo x"}))
	await(t, m, ran.ID, api.RunSucceeded)
	queued := submit(t, m, "q", def("q", "", pipeline.Step{ID: "w", Run: waitFor(filepath.Join(t.TempDir(), "never"))}))
	await(t, m, queued.ID, api.RunRunning)
	for _, id := range []string{ran.ID, queued.ID} {
		if _, err := m.TrackStep(id, api.StepUpdate{ID: "a", Status: api.StepRunning}); !errors.Is(err, ErrConflict) {
			t.Errorf("step of daemon run %s: %v", id, err)
		}
		if _, _, err := m.TrackLog(id, 0, []byte("forged")); !errors.Is(err, ErrConflict) {
			t.Errorf("log of daemon run %s: %v", id, err)
		}
		if _, err := m.TrackFinish(id, api.RunSucceeded, ""); !errors.Is(err, ErrConflict) {
			t.Errorf("finish of daemon run %s: %v", id, err)
		}
	}
}

func TestTrack_NeverTakesASlotOrBlocksAPipeline(t *testing.T) {
	m := open(t, Options{MaxParallel: 1})
	for i := 0; i < 3; i++ {
		if _, err := m.Track("p", graph("a")); err != nil {
			t.Fatal(err)
		}
	}
	// Three local runs of "p" are running; the daemon still runs "p" at once,
	// under the limit of one, and overlap: skip does not see them.
	skipper := def("p", pipeline.OverlapSkip, pipeline.Step{ID: "a", Run: "echo daemon"})
	run := submit(t, m, "p", skipper)
	if done := await(t, m, run.ID, api.RunSucceeded, api.RunSkipped); done.Status != api.RunSucceeded {
		t.Fatalf("run = %+v", done)
	}
	if active, last := m.Summary("p"); active != 3 || last == nil {
		t.Fatalf("summary = %d %+v", active, last)
	}
}

func TestTrack_CancelClosesTheRecordAtOnce(t *testing.T) {
	m := open(t, Options{})
	run, _ := m.Track("p", graph("a", "b"))
	m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepRunning}) //nolint:errcheck // checked through the run below
	got, err := m.Cancel(run.ID)
	if err != nil || got.Status != api.RunCancelled || !strings.Contains(got.Reason, "cancelled from the daemon") {
		t.Fatalf("cancel = %+v, %v", got, err)
	}
	if got.Steps[0].Status != api.StepCancelled || got.Steps[1].Status != api.StepSkipped {
		t.Fatalf("steps = %+v", got.Steps)
	}
	// The run learns of it from the status it gets back.
	if _, status, err := m.TrackLog(run.ID, 0, nil); !errors.Is(err, ErrConflict) || status != "" {
		t.Fatalf("log append after cancel: %q %v", status, err)
	}
	if again, err := m.Cancel(run.ID); err != nil || again.Status != api.RunCancelled {
		t.Fatalf("second cancel = %+v, %v", again, err)
	}
}

func TestTrack_RunLeftRunningByAStoppedDaemonFails(t *testing.T) {
	dir := t.TempDir()
	m := open(t, Options{Dir: dir})
	active, _ := m.Track("p", graph("a"))
	done, _ := m.Track("p", graph("a"))
	if _, err := m.TrackFinish(done.ID, api.RunSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	m.Close() // a graceful stop must not panic on a run it cannot cancel

	again := open(t, Options{Dir: dir})
	got, err := again.Get(active.ID)
	if err != nil || got.Status != api.RunFailed || !strings.Contains(got.Reason, "daemon stopped") {
		t.Fatalf("run = %+v, %v", got, err)
	}
	if kept, _ := again.Get(done.ID); kept.Status != api.RunSucceeded || kept.Trigger != api.TriggerLocal {
		t.Fatalf("finished run = %+v", kept)
	}
	if _, err := again.Track("p", graph("a")); err != nil {
		t.Fatal(err)
	}
}

func TestTrack_KeepsCreationOrderWhenTheClockDoesNotMove(t *testing.T) {
	frozen := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	m := open(t, Options{Now: func() time.Time { return frozen }})
	var ids []string
	for range 6 {
		run, err := m.Track("p", graph("a"))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, run.ID)
	}
	list := m.List("p", 0)
	for i, run := range list {
		if want := ids[len(ids)-1-i]; run.ID != want {
			t.Fatalf("position %d is %s, want %s (newest first)", i, run.ID, want)
		}
	}
}

func TestTrack_RejectsBrokenStepGraphs(t *testing.T) {
	m := open(t, Options{})
	for name, steps := range map[string][]api.PlanStep{
		"unknown need": {{ID: "a", Needs: []string{"zzz"}}},
		"self need":    {{ID: "a", Needs: []string{"a"}}},
		"cycle":        {{ID: "a", Needs: []string{"b"}}, {ID: "b", Needs: []string{"a"}}},
		"bad utf-8":    {{ID: "a\xff"}},
	} {
		if _, err := m.Track("p", steps); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := m.Track("p", []api.PlanStep{{ID: "a"}, {ID: "b", Needs: []string{"a"}}}); err != nil {
		t.Fatalf("a valid graph was refused: %v", err)
	}
}

func TestTrack_ReplacesControlCharactersInReasons(t *testing.T) {
	m := open(t, Options{})
	run, _ := m.Track("p", graph("a"))
	got, err := m.TrackStep(run.ID, api.StepUpdate{ID: "a", Status: api.StepFailed, Error: "bad\x1b[31m\nred"})
	if err != nil || strings.ContainsAny(got.Steps[0].Error, "\x1b\n") {
		t.Fatalf("step error = %q, %v", got.Steps[0].Error, err)
	}
	done, err := m.TrackFinish(run.ID, api.RunFailed, "oops\x07\x1b]0;x\x07")
	if err != nil || strings.ContainsAny(done.Reason, "\x07\x1b") {
		t.Fatalf("reason = %q, %v", done.Reason, err)
	}
}

func TestTrack_LimitsOpenRunsAndLogSize(t *testing.T) {
	m := open(t, Options{Keep: api.MaxOpenTracked + 10})
	var first api.Run
	for i := range api.MaxOpenTracked {
		run, err := m.Track("p", graph("a"))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = run
		}
	}
	if _, err := m.Track("p", graph("a")); !errors.Is(err, ErrConflict) {
		t.Fatalf("one run too many: %v", err)
	}
	if _, err := m.TrackFinish(first.ID, api.RunSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	next, err := m.Track("p", graph("a"))
	if err != nil {
		t.Fatalf("a slot was not freed: %v", err)
	}
	// The log may hold at most api.MaxTrackedLog bytes.
	chunk := make([]byte, api.MaxLogChunk)
	var offset int64
	for offset+int64(len(chunk)) <= api.MaxTrackedLog {
		if offset, _, err = m.TrackLog(next.ID, offset, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.TrackLog(next.ID, offset, chunk); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("log past the limit: %v", err)
	}
}

func TestTrack_RepeatedAppendMustBeIdentical(t *testing.T) {
	m := open(t, Options{})
	run, _ := m.Track("p", graph("a"))
	if _, _, err := m.TrackLog(run.ID, 0, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.TrackLog(run.ID, 0, []byte("jello\n")); !errors.Is(err, ErrConflict) {
		t.Fatalf("different bytes at a held offset: %v", err)
	}
}

func TestQueue_RestoredRunShowsTheStepsOfTheFileItWillRun(t *testing.T) {
	dir := t.TempDir()
	old := def("p", pipeline.OverlapQueue, pipeline.Step{ID: "test", Run: waitFor(filepath.Join(t.TempDir(), "never"))})
	m := open(t, Options{Dir: dir})
	submit(t, m, "p", old)
	queued := submit(t, m, "p", old)
	m.Close()

	edited := def("p", pipeline.OverlapQueue,
		pipeline.Step{ID: "unit", Run: "echo unit"},
		pipeline.Step{ID: "lint", Run: "echo lint", Needs: []string{"unit"}})
	again := open(t, Options{Dir: dir, Resolve: resolveTo(t.TempDir(), map[string]pipeline.Pipeline{"p": edited})})
	got, _ := again.Get(queued.ID)
	if len(got.Steps) != 2 || got.Steps[0].ID != "unit" || got.Steps[1].ID != "lint" || got.Steps[0].Status != api.StepPending {
		t.Fatalf("steps before the run = %+v", got.Steps)
	}
	again.Resume()
	done := await(t, again, queued.ID, api.RunSucceeded, api.RunFailed)
	if done.Status != api.RunSucceeded || done.Steps[0].Status != api.StepSucceeded || done.Steps[1].Status != api.StepSucceeded {
		t.Fatalf("run = %+v", done)
	}
}

func TestTrack_LocalRecordLeftRunningAfterACrashFails(t *testing.T) {
	dir := t.TempDir()
	m := open(t, Options{Dir: dir})
	run, _ := m.Track("p", graph("a"))
	// No Close: the daemon died. A new manager reads what is on disk.
	again := open(t, Options{Dir: dir})
	got, err := again.Get(run.ID)
	if err != nil || got.Status != api.RunFailed || got.Trigger != api.TriggerLocal || got.Steps[0].Status != api.StepSkipped {
		t.Fatalf("run = %+v, %v", got, err)
	}
	if running, _ := again.Counts(); running != 0 {
		t.Fatalf("running = %d", running)
	}
}

func TestTrack_RunningLocalRunCountsAsRunning(t *testing.T) {
	m := open(t, Options{})
	if _, err := m.Track("p", graph("a")); err != nil {
		t.Fatal(err)
	}
	if running, queued := m.Counts(); running != 1 || queued != 0 {
		t.Fatalf("counts = %d running, %d queued", running, queued)
	}
}

func TestTrack_RetriesTolerantFailuresAndSkippedSteps(t *testing.T) {
	m := open(t, Options{})
	run, err := m.Track("local-rich", graph("flaky", "lint", "gated", "after"))
	if err != nil {
		t.Fatal(err)
	}
	step := func(u api.StepUpdate) api.Step {
		t.Helper()
		got, err := m.TrackStep(run.ID, u)
		if err != nil {
			t.Fatalf("%+v: %v", u, err)
		}
		for _, s := range got.Steps {
			if s.ID == u.ID {
				return s
			}
		}
		t.Fatalf("no step %s", u.ID)
		return api.Step{}
	}
	// A failed attempt may start again; the step then describes the new attempt.
	step(api.StepUpdate{ID: "flaky", Status: api.StepRunning})
	if s := step(api.StepUpdate{ID: "flaky", Status: api.StepFailed, ExitCode: exit(1)}); s.ExitCode == nil || *s.ExitCode != 1 {
		t.Fatalf("first attempt = %+v", s)
	}
	if s := step(api.StepUpdate{ID: "flaky", Status: api.StepRunning, Attempts: 2}); s.Status != api.StepRunning || s.Attempts != 2 || s.ExitCode != nil || s.FinishedAt != nil {
		t.Fatalf("retry = %+v", s)
	}
	if s := step(api.StepUpdate{ID: "flaky", Status: api.StepSucceeded, ExitCode: exit(0)}); s.Status != api.StepSucceeded || s.Attempts != 2 {
		t.Fatalf("last attempt = %+v", s)
	}
	// A tolerated failure keeps its exit code and gains the flag.
	step(api.StepUpdate{ID: "lint", Status: api.StepRunning})
	step(api.StepUpdate{ID: "lint", Status: api.StepFailed, ExitCode: exit(3)})
	if s := step(api.StepUpdate{ID: "lint", Status: api.StepFailed, ContinuedOnError: true}); !s.ContinuedOnError || s.ExitCode == nil || *s.ExitCode != 3 {
		t.Fatalf("tolerated = %+v", s)
	}
	// A skipped step carries its reason and never started.
	if s := step(api.StepUpdate{ID: "gated", Status: api.StepSkipped, Reason: "its if condition is false"}); s.Status != api.StepSkipped || s.Reason == "" || s.StartedAt != nil {
		t.Fatalf("skipped = %+v", s)
	}
	// Other transitions of a finished step are still conflicts.
	for _, u := range []api.StepUpdate{
		{ID: "flaky", Status: api.StepRunning},
		{ID: "lint", Status: api.StepSucceeded},
		{ID: "gated", Status: api.StepRunning},
	} {
		if _, err := m.TrackStep(run.ID, u); !errors.Is(err, ErrConflict) {
			t.Errorf("%+v: err = %v, want a conflict", u, err)
		}
	}
}
