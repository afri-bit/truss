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
