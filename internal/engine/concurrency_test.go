package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

// Spec 004 (step concurrency): PR-B tests T015 (US1), T021 (US3), T030 (US5).
// Rendezvous between Steps uses files and the helper process, so a scheduler
// that runs Steps one at a time fails by a deadline instead of hanging.

// shortDeadlineMS is the rendezvous deadline for negative cases that must
// fail fast. It only bounds how long a wrong implementation takes to fail.
const shortDeadlineMS = "300"

func runWithLimit(t *testing.T, p pipeline.Pipeline, limit int) (pipeline.PipelineRun, *eventLog) {
	t.Helper()
	p.Concurrency = limit
	rec := &eventLog{}
	run, err := Run(context.Background(), p, t.TempDir(), nil, rec)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return run, rec
}

// pos returns the index of the first event of kind for id, or -1.
func pos(rec *eventLog, kind, id string) int {
	for i, ev := range rec.events {
		if ev.kind == kind && ev.id == id {
			return i
		}
	}
	return -1
}

// maxRunning derives the highest number of Steps that were running at once
// from the started and finished events.
func maxRunning(rec *eventLog) int {
	running, highest := 0, 0
	for _, ev := range rec.events {
		switch ev.kind {
		case "started":
			running++
			if running > highest {
				highest = running
			}
		case "finished":
			running--
		}
	}
	return highest
}

func startOrder(rec *eventLog) []string {
	var ids []string
	for _, ev := range rec.events {
		if ev.kind == "started" {
			ids = append(ids, ev.id)
		}
	}
	return ids
}

func runIDs(run pipeline.PipelineRun) []string {
	var ids []string
	for _, s := range run.Steps {
		ids = append(ids, s.ID)
	}
	return ids
}

func stepRun(t *testing.T, run pipeline.PipelineRun, id string) pipeline.StepRun {
	t.Helper()
	for _, s := range run.Steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("Step %q is not in run.Steps %v", id, runIDs(run))
	return pipeline.StepRun{}
}

func requireOrder(t *testing.T, rec *eventLog, before, after [2]string) {
	t.Helper()
	b, a := pos(rec, before[0], before[1]), pos(rec, after[0], after[1])
	if b < 0 || a < 0 || b >= a {
		t.Fatalf("want %s %s before %s %s; positions %d, %d; events %q", before[0], before[1], after[0], after[1], b, a, rec.phases())
	}
}

func requireSucceeded(t *testing.T, run pipeline.PipelineRun) {
	t.Helper()
	if run.Status != pipeline.StatusSucceeded {
		t.Fatalf("status = %s, want succeeded; run = %+v", run.Status, run)
	}
	for _, s := range run.Steps {
		if s.Status != pipeline.StatusSucceeded {
			t.Fatalf("Step %s = %+v", s.ID, s)
		}
	}
}

func okStep(id string, needs ...string) pipeline.Step {
	return pipeline.Step{ID: id, Run: helperCmd("exit", "0"), Needs: needs}
}

// rendezvousStep touches its own file and waits for other's.
func rendezvousStep(dir, id, other string, needs ...string) pipeline.Step {
	return pipeline.Step{
		ID:    id,
		Run:   helperCmd("touch-and-wait", filepath.Join(dir, id), filepath.Join(dir, other)),
		Needs: needs,
	}
}

// P1-1.
func TestRun_ConcurrentLimitTwoRunsIndependentStepsTogether(t *testing.T) {
	dir := t.TempDir()
	p := pipeline.Pipeline{Version: "1", Name: "pair", Steps: []pipeline.Step{
		rendezvousStep(dir, "a", "b"),
		rendezvousStep(dir, "b", "a"),
	}}
	run, rec := runWithLimit(t, p, 2)
	requireSucceeded(t, run)
	if len(run.Steps) != 2 {
		t.Fatalf("steps = %v", runIDs(run))
	}
	requireOrder(t, rec, [2]string{"started", "a"}, [2]string{"finished", "a"})
	requireOrder(t, rec, [2]string{"started", "a"}, [2]string{"finished", "b"})
	requireOrder(t, rec, [2]string{"started", "b"}, [2]string{"finished", "a"})
	requireOrder(t, rec, [2]string{"started", "b"}, [2]string{"finished", "b"})
	if got := maxRunning(rec); got != 2 {
		t.Fatalf("max running = %d, want 2", got)
	}
}

// P1-1: with limit 1, or with the field omitted, the same pair cannot meet.
func TestRun_ConcurrentLimitOneOrOmittedRunsOneAtATime(t *testing.T) {
	for _, limit := range []int{0, 1} {
		t.Run(fmt.Sprintf("limit %d", limit), func(t *testing.T) {
			dir := t.TempDir()
			wait := func(id, other string) pipeline.Step {
				return pipeline.Step{ID: id, Run: helperCmd("touch-and-wait", filepath.Join(dir, id), filepath.Join(dir, other), shortDeadlineMS)}
			}
			p := pipeline.Pipeline{Version: "1", Name: "pair", Steps: []pipeline.Step{wait("a", "b"), wait("b", "a")}}
			start := time.Now()
			run, rec := runWithLimit(t, p, limit)
			if run.Status != pipeline.StatusFailed {
				t.Fatalf("status = %s, want failed (a waits for b, which never starts)", run.Status)
			}
			if got := runIDs(run); strings.Join(got, ",") != "a" {
				t.Fatalf("steps = %v, want only a", got)
			}
			if got := maxRunning(rec); got != 1 {
				t.Fatalf("max running = %d, want 1", got)
			}
			if time.Since(start) > 3*time.Second {
				t.Fatalf("took %s", time.Since(start))
			}
		})
	}
}

// P1-2.
func TestRun_ConcurrentThirdStepStartsOnlyWhenASlotFrees(t *testing.T) {
	dir := t.TempDir()
	p := pipeline.Pipeline{Version: "1", Name: "three", Steps: []pipeline.Step{
		rendezvousStep(dir, "a", "b"),
		rendezvousStep(dir, "b", "a"),
		okStep("c"),
	}}
	run, rec := runWithLimit(t, p, 2)
	requireSucceeded(t, run)
	if got := startOrder(rec); strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("start order = %v", got)
	}
	c := pos(rec, "started", "c")
	fa, fb := pos(rec, "finished", "a"), pos(rec, "finished", "b")
	if c < min(fa, fb) {
		t.Fatalf("c started before a or b finished; events %q", rec.phases())
	}
	if got := maxRunning(rec); got != 2 {
		t.Fatalf("max running = %d, want 2 (the limit, never exceeded)", got)
	}
}

// P1-3.
func TestRun_ConcurrentDependentStartsAfterItsNeedNotAfterUnrelatedStep(t *testing.T) {
	dir := t.TempDir()
	// lint and test wait for each other, so test must start while lint runs.
	p := pipeline.Pipeline{Version: "1", Name: "ci", Steps: []pipeline.Step{
		okStep("build"),
		rendezvousStep(dir, "lint", "test"),
		rendezvousStep(dir, "test", "lint", "build"),
	}}
	run, rec := runWithLimit(t, p, 3)
	requireSucceeded(t, run)
	if got := startOrder(rec); len(got) != 3 || got[0] != "build" || got[1] != "lint" || got[2] != "test" {
		t.Fatalf("start order = %v, want build, lint, test", got)
	}
	requireOrder(t, rec, [2]string{"finished", "build"}, [2]string{"started", "test"})
	requireOrder(t, rec, [2]string{"started", "test"}, [2]string{"finished", "lint"})
	requireOrder(t, rec, [2]string{"started", "lint"}, [2]string{"finished", "test"})
}

// P1-4.
func TestRun_ConcurrentTwoChainsAtLimitTwo(t *testing.T) {
	dir := t.TempDir()
	p := pipeline.Pipeline{Version: "1", Name: "chains", Steps: []pipeline.Step{
		rendezvousStep(dir, "a", "c"),
		okStep("b", "a"),
		rendezvousStep(dir, "c", "a"),
		okStep("d", "c"),
	}}
	run, rec := runWithLimit(t, p, 2)
	requireSucceeded(t, run)
	if got := startOrder(rec); got[0] != "a" || got[1] != "c" {
		t.Fatalf("start order = %v, want a and c first", got)
	}
	requireOrder(t, rec, [2]string{"finished", "a"}, [2]string{"started", "b"})
	requireOrder(t, rec, [2]string{"finished", "c"}, [2]string{"started", "d"})
	if got := maxRunning(rec); got > 2 {
		t.Fatalf("max running = %d, want at most 2", got)
	}
}

// P1-5: a limit above the number of ready Steps just runs the ready ones.
func TestRun_ConcurrentLimitAboveReadyStepsRunsThemAll(t *testing.T) {
	dir := t.TempDir()
	// A ring: each Step waits for the next, so all three must run together.
	p := pipeline.Pipeline{Version: "1", Name: "ring", Steps: []pipeline.Step{
		rendezvousStep(dir, "a", "b"),
		rendezvousStep(dir, "b", "c"),
		rendezvousStep(dir, "c", "a"),
	}}
	run, rec := runWithLimit(t, p, 100)
	requireSucceeded(t, run)
	if got := maxRunning(rec); got != 3 {
		t.Fatalf("max running = %d, want 3", got)
	}
	for _, id := range []string{"a", "b", "c"} {
		requireOrder(t, rec, [2]string{"started", id}, [2]string{"finished", "a"})
		requireOrder(t, rec, [2]string{"started", id}, [2]string{"finished", "b"})
		requireOrder(t, rec, [2]string{"started", id}, [2]string{"finished", "c"})
	}
}

// P1-6.
func TestRun_ConcurrentStrictChainRunsOneAtATimeInOrder(t *testing.T) {
	p := pipeline.Pipeline{Version: "1", Name: "chain", Steps: []pipeline.Step{
		okStep("a"), okStep("b", "a"), okStep("c", "b"),
	}}
	run, rec := runWithLimit(t, p, 2)
	requireSucceeded(t, run)
	var seq []string
	for _, ev := range rec.events {
		if ev.kind == "started" || ev.kind == "finished" {
			seq = append(seq, ev.kind+" "+ev.id)
		}
	}
	want := "started a,finished a,started b,finished b,started c,finished c"
	if strings.Join(seq, ",") != want {
		t.Fatalf("events = %q, want %q", seq, want)
	}
	if got := runIDs(run); strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("steps = %v", got)
	}
}

// --- T021: the first failure stops running Steps (US3) ---

// startsAfterFirstFailure lists Steps that started after the first failed
// finish. Exit code 0 is the only success, so every finished event with a
// non-zero code or a diagnostic is a failure.
func startsAfterFirstFailure(rec *eventLog) []string {
	failed := false
	var late []string
	for _, ev := range rec.events {
		switch {
		case ev.kind == "finished" && (ev.exit == nil || *ev.exit != 0):
			failed = true
		case ev.kind == "started" && failed:
			late = append(late, ev.id)
		}
	}
	return late
}

func TestRun_StopEndsRunningStepAndItsGrandchildAfterFailure(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	p := pipeline.Pipeline{Version: "1", Name: "stop", Steps: []pipeline.Step{
		// a fails once b's grandchild exists, so b is certainly running.
		{ID: "a", Run: helperCmd("exit-after", pidFile, "1")},
		{ID: "b", Run: helperCmd("spawn-grandchild", pidFile)},
		okStep("c"),
	}}
	run, rec := runWithLimit(t, p, 2)

	if run.Status != pipeline.StatusFailed || run.FailedStep != "a" {
		t.Fatalf("status = %s, FailedStep = %q, want failed, a", run.Status, run.FailedStep)
	}
	if got := runIDs(run); strings.Join(got, ",") != "a,b" {
		t.Fatalf("steps = %v, want a,b (c never started)", got)
	}
	if pos(rec, "started", "c") >= 0 {
		t.Fatalf("c started; events %q", rec.phases())
	}
	a, b := stepRun(t, run, "a"), stepRun(t, run, "b")
	if a.ExitCode == nil || *a.ExitCode != 1 || a.StoppedBy != "" {
		t.Fatalf("a = %+v, want its own exit 1 and not stopped", a)
	}
	if b.Status != pipeline.StatusFailed || b.StoppedBy != "a" {
		t.Fatalf("b = %+v, want failed and stopped by a", b)
	}
	stopped := pos(rec, "stopped", "b")
	if stopped < 0 || rec.events[stopped].chunk != "a" {
		t.Fatalf("no StepStopped(b, a); events %q", rec.phases())
	}
	if pos(rec, "stopped", "a") >= 0 {
		t.Fatalf("a was reported stopped; events %q", rec.phases())
	}
	if stopped > pos(rec, "finished", "b") {
		t.Fatalf("StepStopped after StepFinished for b; events %q", rec.phases())
	}
	for i, ev := range rec.events {
		if ev.kind == "output" && ev.id == "b" && i > stopped {
			t.Fatalf("output of b after StepStopped; events %q", rec.phases())
		}
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("grandchild pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("grandchild pid %q: %v", data, err)
	}
	requireGone(t, "grandchild of b", pid)
}

// requireGone checks the process is gone once Run has returned. Unix ends the
// tree before the Step is reported finished; Windows termination is
// effectively synchronous, so a bounded poll is allowed there.
func requireGone(t *testing.T, what string, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for pidAlive(pid) {
		if runtime.GOOS != "windows" || time.Now().After(deadline) {
			t.Fatalf("%s (pid %d) is still alive after Run returned", what, pid)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A Step that ends by a signal has a diagnostic and no Exit Code; it is the
// first failure and the other running Step is stopped.
func TestRun_StopAfterStepEndedBySignal(t *testing.T) {
	if !signalStepSupported {
		t.Skip("Windows has no signals; the no-Exit-Code diagnostic path is covered on Unix")
	}
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	p := pipeline.Pipeline{Version: "1", Name: "signalled", Steps: []pipeline.Step{
		{ID: "a", Run: signalStepRun(ready)},
		{ID: "b", Run: helperCmd("touch-and-wait", ready, filepath.Join(dir, "never"))},
	}}
	run, rec := runWithLimit(t, p, 2)

	if run.Status != pipeline.StatusFailed || run.FailedStep != "a" {
		t.Fatalf("status = %s, FailedStep = %q, want failed, a", run.Status, run.FailedStep)
	}
	a, b := stepRun(t, run, "a"), stepRun(t, run, "b")
	if a.ExitCode != nil || a.Error == "" || a.StoppedBy != "" {
		t.Fatalf("a = %+v, want a diagnostic, no Exit Code, not stopped", a)
	}
	if b.Status != pipeline.StatusFailed || b.StoppedBy != "a" {
		t.Fatalf("b = %+v, want stopped by a", b)
	}
	requireOrder(t, rec, [2]string{"stopped", "b"}, [2]string{"finished", "b"})
}

// Two Steps fail at about the same time: each keeps its own result and
// FailedStep names the one whose failure the scheduler saw first.
func TestRun_StopTwoFailuresKeepTheirOwnExitCodes(t *testing.T) {
	dir := t.TempDir()
	p := pipeline.Pipeline{Version: "1", Name: "both-fail", Steps: []pipeline.Step{
		{ID: "a", Run: helperCmd("rendezvous-exit", filepath.Join(dir, "a"), filepath.Join(dir, "b"), "3")},
		{ID: "b", Run: helperCmd("rendezvous-exit", filepath.Join(dir, "b"), filepath.Join(dir, "a"), "4")},
	}}
	run, rec := runWithLimit(t, p, 2)

	if run.Status != pipeline.StatusFailed || len(run.Steps) != 2 {
		t.Fatalf("run = %+v", run)
	}
	var first string
	for _, ev := range rec.events {
		if ev.kind == "finished" {
			first = ev.id
			break
		}
	}
	if run.FailedStep != first {
		t.Fatalf("FailedStep = %q, want %q (first finished event)", run.FailedStep, first)
	}
	own := map[string]int{"a": 3, "b": 4}
	for id, code := range own {
		s := stepRun(t, run, id)
		if s.Status != pipeline.StatusFailed {
			t.Fatalf("%s = %+v", id, s)
		}
		if s.StoppedBy == "" {
			if s.ExitCode == nil || *s.ExitCode != code {
				t.Fatalf("%s = %+v, want its own exit %d", id, s, code)
			}
			continue
		}
		// The stop reached it before it exited: allowed, but then it must
		// be stopped by the first failure and reported as such.
		if id == first || s.StoppedBy != first || pos(rec, "stopped", id) < 0 {
			t.Fatalf("%s = %+v, first = %s", id, s, first)
		}
	}
}

// A Step that exits 0 by itself while the stop races is succeeded and is not
// reported stopped (contracts/execution.md).
func TestRun_StopStepThatExitsZeroWhileStopRacesIsSucceeded(t *testing.T) {
	for i := 0; i < 20; i++ {
		dir := t.TempDir()
		fa, fb := filepath.Join(dir, "a"), filepath.Join(dir, "b")
		p := pipeline.Pipeline{Version: "1", Name: "race", Steps: []pipeline.Step{
			{ID: "a", Run: helperCmd("rendezvous-exit", fa, fb, "1")},
			{ID: "b", Run: helperCmd("rendezvous-exit", fb, fa, "0")},
		}}
		run, rec := runWithLimit(t, p, 2)

		if run.Status != pipeline.StatusFailed || run.FailedStep != "a" {
			t.Fatalf("iteration %d: status = %s, FailedStep = %q", i, run.Status, run.FailedStep)
		}
		b := stepRun(t, run, "b")
		stopped := pos(rec, "stopped", "b") >= 0
		if b.ExitCode != nil && *b.ExitCode == 0 {
			if b.Status != pipeline.StatusSucceeded || b.StoppedBy != "" || stopped {
				t.Fatalf("iteration %d: b exited 0 by itself but b = %+v, stopped event = %v", i, b, stopped)
			}
			continue
		}
		// The stop won the race: then b must say so.
		if b.Status != pipeline.StatusFailed || b.StoppedBy != "a" || !stopped {
			t.Fatalf("iteration %d: b = %+v, stopped event = %v", i, b, stopped)
		}
	}
}

// After a failure no Step starts, even one whose Dependencies are met.
func TestRun_StopStartsNothingAfterAFailureEvenWhenNeedsAreMet(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "b-ran")
	p := pipeline.Pipeline{Version: "1", Name: "no-more", Steps: []pipeline.Step{
		{ID: "a", Run: helperCmd("exit-after", f, "1")},
		{ID: "b", Run: helperCmd("touch-and-wait", f, f)},
		okStep("c", "b"),
		okStep("d"),
	}}
	run, rec := runWithLimit(t, p, 2)

	if run.Status != pipeline.StatusFailed || run.FailedStep != "a" {
		t.Fatalf("status = %s, FailedStep = %q", run.Status, run.FailedStep)
	}
	if late := startsAfterFirstFailure(rec); len(late) > 0 {
		t.Fatalf("Steps started after the failure: %v; events %q", late, rec.phases())
	}
	started := map[string]bool{}
	for _, id := range startOrder(rec) {
		started[id] = true
	}
	for _, s := range run.Steps {
		if !started[s.ID] {
			t.Fatalf("run.Steps has %s, which never started", s.ID)
		}
	}
	if len(run.Steps) != len(started) {
		t.Fatalf("run.Steps = %v, started = %v", runIDs(run), startOrder(rec))
	}
}

// --- T030: limit omitted and limit 1 follow the spec 003 order (US5) ---

// The definitions below are the spec 003 order tables from
// internal/pipeline/plan_test.go (TestPlan_DefinitionOrder, _DependencyOrder,
// _WithoutNeedsKeepsDefinitionOrder), copied here on purpose.
var spec003Tables = []struct {
	name  string
	steps []pipeline.Step
}{
	{"definition order", []pipeline.Step{{ID: "hello"}, {ID: "second"}}},
	{"earlier step needs a later step", []pipeline.Step{
		{ID: "test", Needs: []string{"build"}}, {ID: "build"},
	}},
	{"step needs two later steps", []pipeline.Step{
		{ID: "package", Needs: []string{"lint", "test"}}, {ID: "lint"}, {ID: "test"},
	}},
	{"unrelated last step stays last", []pipeline.Step{
		{ID: "report", Needs: []string{"build"}}, {ID: "build"}, {ID: "scan"},
	}},
	{"steps without needs are not ordered after their predecessor", []pipeline.Step{
		{ID: "publish", Needs: []string{"package"}}, {ID: "lint"}, {ID: "package", Needs: []string{}},
	}},
	{"order of ids inside needs does not change run order", []pipeline.Step{
		{ID: "package", Needs: []string{"test", "lint"}}, {ID: "lint"}, {ID: "test"},
	}},
	{"no needs keeps definition order", []pipeline.Step{
		{ID: "c"}, {ID: "a"}, {ID: "b"},
	}},
}

// specPipeline copies the table, gives each Step a command, and makes
// failID (when set) exit 3.
func specPipeline(steps []pipeline.Step, failID string) pipeline.Pipeline {
	out := make([]pipeline.Step, len(steps))
	for i, s := range steps {
		s.Run = "echo " + s.ID
		if s.ID == failID {
			s.Run = helperCmd("exit", "3")
		}
		out[i] = s
	}
	return pipeline.Pipeline{Version: "1", Name: "spec003", Steps: out}
}

// canon renders every event with its payload so two runs can be compared.
func canon(rec *eventLog) string {
	var b strings.Builder
	for _, ev := range rec.events {
		code := "-"
		if ev.exit != nil {
			code = strconv.Itoa(*ev.exit)
		}
		fmt.Fprintf(&b, "%s|%s|%q|%s\n", ev.kind, ev.id, ev.chunk, code)
	}
	return b.String()
}

func TestRun_LimitOneAndOmittedStartInPlanOrder(t *testing.T) {
	for _, tt := range spec003Tables {
		t.Run(tt.name, func(t *testing.T) {
			p := specPipeline(tt.steps, "")
			plan, err := pipeline.Plan(p)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			var want []string
			for _, s := range plan.Steps {
				want = append(want, s.ID)
			}

			omitted, recOmitted := runWithLimit(t, p, 0)
			one, recOne := runWithLimit(t, p, 1)

			for name, got := range map[string]*eventLog{"omitted": recOmitted, "limit 1": recOne} {
				if order := startOrder(got); strings.Join(order, ",") != strings.Join(want, ",") {
					t.Fatalf("%s: start order = %v, want %v", name, order, want)
				}
				if m := maxRunning(got); m != 1 {
					t.Fatalf("%s: max running = %d, want 1", name, m)
				}
			}
			requireSucceeded(t, omitted)
			requireSucceeded(t, one)
			if strings.Join(runIDs(one), ",") != strings.Join(want, ",") || strings.Join(runIDs(omitted), ",") != strings.Join(want, ",") {
				t.Fatalf("run.Steps = %v and %v, want %v", runIDs(omitted), runIDs(one), want)
			}
			if a, b := canon(recOmitted), canon(recOne); a != b {
				t.Fatalf("events differ between omitted and limit 1:\nomitted:\n%s\nlimit 1:\n%s", a, b)
			}
		})
	}
}

// Fail-fast at limit 1 is exactly spec 003: the second planned Step fails,
// everything after it is absent.
func TestRun_LimitOneAndOmittedFailFastOmitsLaterSteps(t *testing.T) {
	for _, tt := range spec003Tables {
		for _, limit := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/limit %d", tt.name, limit), func(t *testing.T) {
				plan, err := pipeline.Plan(specPipeline(tt.steps, ""))
				if err != nil {
					t.Fatalf("Plan() error = %v", err)
				}
				failID := plan.Steps[1].ID
				run, rec := runWithLimit(t, specPipeline(tt.steps, failID), limit)

				want := []string{plan.Steps[0].ID, failID}
				if got := runIDs(run); strings.Join(got, ",") != strings.Join(want, ",") {
					t.Fatalf("run.Steps = %v, want %v", got, want)
				}
				if got := startOrder(rec); strings.Join(got, ",") != strings.Join(want, ",") {
					t.Fatalf("start order = %v, want %v", got, want)
				}
				if run.Status != pipeline.StatusFailed || run.FailedStep != failID {
					t.Fatalf("status = %s, FailedStep = %q, want failed, %s", run.Status, run.FailedStep, failID)
				}
				failed := stepRun(t, run, failID)
				if failed.ExitCode == nil || *failed.ExitCode != 3 || failed.StoppedBy != "" {
					t.Fatalf("%s = %+v", failID, failed)
				}
				if pos(rec, "stopped", failID) >= 0 {
					t.Fatalf("a Step that failed by itself was reported stopped: %q", rec.phases())
				}
			})
		}
	}
}
