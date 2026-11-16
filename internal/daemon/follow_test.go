package daemon

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

// Spec 019: a registered file is the source of truth. The daemon notices an
// edit, shows the file's current name, and keeps the history of the flow.

func namedDefinition(name string, steps ...string) string {
	body := "version: \"1\"\npipeline:\n  name: " + name + "\nsteps:\n"
	for _, s := range steps {
		body += "  - id: " + s + "\n    run: echo " + s + "\n"
	}
	return body
}

// awaitPipeline polls until the daemon lists exactly one pipeline that satisfies ok.
func awaitPipeline(t *testing.T, c *api.Client, what string, ok func(api.Pipeline) bool) api.Pipeline {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		list, err := c.Pipelines(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(list) == 1 && ok(list[0]) {
			return list[0]
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: pipelines = %+v", what, list)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func mustTrigger(t *testing.T, c *api.Client, name string) api.Run {
	t.Helper()
	run, err := c.Trigger(context.Background(), name)
	if err != nil {
		t.Fatalf("trigger %s: %v", name, err)
	}
	return run
}

func pipelinesOf(t *testing.T, c *api.Client) []api.Pipeline {
	t.Helper()
	list, err := c.Pipelines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func runsOf(t *testing.T, c *api.Client, name string) []api.Run {
	t.Helper()
	list, err := c.Runs(context.Background(), name, 0)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func stepIDs(p api.Pipeline) string {
	var ids []string
	for _, s := range p.Steps {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ",")
}

func TestDaemon_FlowFollowsAnEditOfItsFile(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	old, err := c.Trigger(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "alpha", 1)

	if err := os.WriteFile(path, []byte(namedDefinition("alpha", "one", "two")), 0o600); err != nil {
		t.Fatal(err)
	}
	p := awaitPipeline(t, c, "new steps", func(p api.Pipeline) bool { return stepIDs(p) == "one,two" })
	if p.Name != "alpha" {
		t.Fatalf("name = %q, want alpha", p.Name)
	}
	run, err := c.Trigger(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "alpha", 2)
	got, err := c.Run(ctx, run.ID)
	if err != nil || len(got.Steps) != 2 {
		t.Fatalf("new run = %+v, %v; want the two steps of the edited file", got, err)
	}
	if first, err := c.Run(ctx, old.ID); err != nil || len(first.Steps) != 1 {
		t.Fatalf("old run = %+v, %v; history must not change", first, err)
	}
}

func TestDaemon_FlowFollowsItsNameInTheFileAndKeepsItsHistory(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	old, err := c.Trigger(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "alpha", 1)

	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	p := awaitPipeline(t, c, "renamed", func(p api.Pipeline) bool { return p.Name == "beta" })
	if p.RegisteredName != "alpha" || stepIDs(p) != "a,b" || !p.Valid {
		t.Fatalf("pipeline = %+v, want beta (registered as alpha) with steps a,b", p)
	}

	// The history stays, under the new name, and the old name still finds it.
	for _, name := range []string{"beta", "alpha"} {
		list, err := c.Runs(ctx, name, 0)
		if err != nil || len(list) != 1 || list[0].ID != old.ID || list[0].Pipeline != "beta" {
			t.Fatalf("runs of %s = %+v, %v; want the earlier run, shown as beta", name, list, err)
		}
	}
	if got, err := c.Run(ctx, old.ID); err != nil || got.Pipeline != "beta" {
		t.Fatalf("earlier run = %+v, %v; want it shown as beta", got, err)
	}
	if p.LastRun == nil || p.LastRun.ID != old.ID {
		t.Fatalf("last run = %+v, want the earlier run", p.LastRun)
	}

	// Both names start the edited flow and every command accepts either.
	for _, name := range []string{"beta", "alpha"} {
		run, err := c.Trigger(ctx, name)
		if err != nil || run.Pipeline != "beta" {
			t.Fatalf("trigger %s = %+v, %v", name, run, err)
		}
	}
	awaitRuns(t, c, "beta", 3)
	if list := runsOf(t, c, ""); len(list) != 3 {
		t.Fatalf("runs = %d, want 3 in one history", len(list))
	}
	if q, err := c.SetPaused(ctx, "alpha", true); err != nil || q.Name != "beta" || !q.Paused {
		t.Fatalf("pause by the old name = %+v, %v", q, err)
	}
	if q, err := c.SetPaused(ctx, "beta", false); err != nil || q.Paused {
		t.Fatalf("resume by the new name = %+v, %v", q, err)
	}

	// Registering the file again does not make a second flow.
	again, err := c.Register(ctx, api.RegisterRequest{Path: path})
	if err != nil || again.Name != "beta" {
		t.Fatalf("register again = %+v, %v", again, err)
	}
	if list := pipelinesOf(t, c); len(list) != 1 {
		t.Fatalf("pipelines = %d, want 1", len(list))
	}

	// Naming it back drops the difference.
	if err := os.WriteFile(path, []byte(namedDefinition("alpha", "a")), 0o600); err != nil {
		t.Fatal(err)
	}
	back := awaitPipeline(t, c, "renamed back", func(p api.Pipeline) bool { return p.Name == "alpha" })
	if back.RegisteredName != "" {
		t.Fatalf("registered name = %q, want none when it matches", back.RegisteredName)
	}
	if list := runsOf(t, c, "alpha"); len(list) != 3 {
		t.Fatalf("runs = %d, want the whole history of 3", len(list))
	}

	// Unregistering works by the name on screen.
	if err := c.Unregister(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
}

func TestDaemon_ANameAnotherFlowHoldsIsNotTaken(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	one := writeDefinition(t, t.TempDir(), "one.yaml", namedDefinition("one", "x"))
	two := writeDefinition(t, t.TempDir(), "two.yaml", namedDefinition("two", "x"))
	for _, path := range []string{one, two} {
		if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(two, []byte(namedDefinition("one", "x", "y")), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		list, err := c.Pipelines(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) == 2 && stepIDs(list[1]) == "x,y" {
			if list[0].Name != "one" || list[1].Name != "two" || list[1].DefinitionName != "one" {
				t.Fatalf("pipelines = %+v; the second keeps its registered name while the first holds one", list)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipelines = %+v", list)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if run, err := c.Trigger(ctx, "one"); err != nil || len(run.Steps) != 1 {
		t.Fatalf("trigger one = %+v, %v; want the first file", run, err)
	}
}

func TestDaemon_ANameChosenAtRegistrationIsKept(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path, Name: "mine"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	p := awaitPipeline(t, c, "edited", func(p api.Pipeline) bool { return stepIDs(p) == "a,b" })
	if p.Name != "mine" || p.RegisteredName != "" || p.DefinitionName != "beta" {
		t.Fatalf("pipeline = %+v; a name chosen with --name is not replaced by the file's", p)
	}
	if _, err := c.Trigger(ctx, "beta"); err == nil {
		t.Fatal("the file's name must not answer for a flow registered under a chosen name")
	}
}

// A file that stops loading after a rename keeps the name it was shown under,
// so the name on screen and in scripts does not jump back (FR-001, FR-003).
func TestDaemon_ABrokenFileKeepsItsShownNameAndHistory(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	old := mustTrigger(t, c, "alpha")
	awaitRuns(t, c, "alpha", 1)
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a")), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "renamed", func(p api.Pipeline) bool { return p.Name == "beta" })

	if err := os.WriteFile(path, []byte("version: \"1\"\npipeline:\n  name: beta\nsteps: [this is: not valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := awaitPipeline(t, c, "invalid", func(p api.Pipeline) bool { return !p.Valid })
	if p.Name != "beta" || p.RegisteredName != "alpha" || p.Error == "" {
		t.Fatalf("pipeline = %+v, want beta shown as invalid with the reason", p)
	}
	if list, err := c.Runs(ctx, "beta", 0); err != nil || len(list) != 1 || list[0].ID != old.ID {
		t.Fatalf("history = %+v, %v; want it kept while the file is broken", list, err)
	}
}

// Registering a file again without --name never makes a second registration,
// whatever name its file gives (FR-008).
func TestDaemon_RegisteringTheSameFileAgainMakesNoSecondRegistration(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path, Name: "mine"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "edited", func(p api.Pipeline) bool { return stepIDs(p) == "a,b" })
	again, err := c.Register(ctx, api.RegisterRequest{Path: path})
	if err != nil || again.Name != "mine" {
		t.Fatalf("register again = %+v, %v; want the existing registration", again, err)
	}
	if list := pipelinesOf(t, c); len(list) != 1 {
		t.Fatalf("pipelines = %d, want 1", len(list))
	}
	// A name chosen for another entry is still allowed: the same file may have several.
	if second, err := c.Register(ctx, api.RegisterRequest{Path: path, Name: "second"}); err != nil || second.Name != "second" {
		t.Fatalf("second name = %+v, %v", second, err)
	}
}

// The same holds when two files claim one new name and neither shows it.
func TestDaemon_ATieOverANewNameKeepsBothRegisteredNamesAndMakesNoDuplicate(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	a := writeDefinition(t, t.TempDir(), "a.yaml", namedDefinition("alpha", "x"))
	g := writeDefinition(t, t.TempDir(), "g.yaml", namedDefinition("gamma", "x"))
	for _, path := range []string{a, g} {
		if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{a, g} {
		if err := os.WriteFile(path, []byte(namedDefinition("beta", "x", "y")), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		list := pipelinesOf(t, c)
		if len(list) == 2 && stepIDs(list[0]) == "x,y" && stepIDs(list[1]) == "x,y" {
			if list[0].Name != "alpha" || list[1].Name != "gamma" {
				t.Fatalf("pipelines = %+v; a contested name belongs to neither", list)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipelines = %+v", list)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := c.Trigger(ctx, "beta"); err == nil {
		t.Fatal("a contested name must answer for nothing")
	}
	if again, err := c.Register(ctx, api.RegisterRequest{Path: a}); err != nil || again.Name != "alpha" {
		t.Fatalf("register a again = %+v, %v", again, err)
	}
	if list := pipelinesOf(t, c); len(list) != 2 {
		t.Fatalf("pipelines = %d, want 2: the same file must not register twice", len(list))
	}
}

// Registering again with --name keeps that name from then on (the remedy for
// a registration made before names followed files).
func TestDaemon_RegisteringAgainWithItsNameKeepsTheName(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path, Name: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	p := awaitPipeline(t, c, "edited", func(p api.Pipeline) bool { return stepIDs(p) == "a,b" })
	if p.Name != "alpha" || p.RegisteredName != "" {
		t.Fatalf("pipeline = %+v; the name chosen again must stay", p)
	}
}

// A name a flow shows is held: unregister works by it, and another file cannot take it (409).
func TestDaemon_AShownNameIsHeldAndWorksToUnregister(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "renamed", func(p api.Pipeline) bool { return p.Name == "beta" })

	other := writeDefinition(t, t.TempDir(), "other.yaml", namedDefinition("other", "x"))
	var apiErr *api.StatusError
	if _, err := c.Register(ctx, api.RegisterRequest{Path: other, Name: "beta"}); !errors.As(err, &apiErr) || apiErr.Code != http.StatusConflict {
		t.Fatalf("register under a shown name: %v, want 409", err)
	}
	if err := c.Unregister(ctx, "beta"); err != nil {
		t.Fatalf("unregister by the shown name: %v", err)
	}
	if list := pipelinesOf(t, c); len(list) != 0 {
		t.Fatalf("pipelines = %+v, want none", list)
	}
	if _, err := c.Trigger(ctx, "alpha"); !errors.As(err, &apiErr) || apiErr.Code != http.StatusNotFound {
		t.Fatalf("trigger after unregister: %v, want 404", err)
	}
}

// A run names the registration it belongs to as well as the name shown, so a
// client can act on the registration even if a shown name moves (security 019).
func TestDaemon_RunsNameTheirRegistrationAndTrackedRunsJoinTheHistory(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	first := mustTrigger(t, c, "alpha")
	awaitRuns(t, c, "alpha", 1)
	if first.RegisteredPipeline != "" {
		t.Fatalf("registered pipeline = %q, want none while the names match", first.RegisteredPipeline)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "renamed", func(p api.Pipeline) bool { return p.Name == "beta" })

	got, err := c.Run(ctx, first.ID)
	if err != nil || got.Pipeline != "beta" || got.RegisteredPipeline != "alpha" {
		t.Fatalf("run = %+v, %v; want beta, registered as alpha", got, err)
	}
	tracked, err := c.CreateRun(ctx, api.CreateRunRequest{Pipeline: "beta", Steps: []api.PlanStep{{ID: "a"}}})
	if err != nil || tracked.Pipeline != "beta" || tracked.RegisteredPipeline != "" {
		t.Fatalf("tracked run = %+v, %v; a local run is not registered", tracked, err)
	}
	list, err := c.Runs(ctx, "beta", 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("history = %+v, %v; want the run and the tracked run", list, err)
	}
	if p := awaitPipeline(t, c, "summary", func(p api.Pipeline) bool { return p.ActiveRuns == 1 }); p.LastRun == nil {
		t.Fatalf("pipeline = %+v; the tracked run is active and counts", p)
	}
}

// The daemon log says when it read a changed file again (FR-009), also when a
// run trigger is the first to see the edit, and does not repeat itself for a
// file that stays broken.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestDaemon_LogsAChangedFileOnceUntilItChangesAgain(t *testing.T) {
	ctx := context.Background()
	var log lockedBuffer
	h := home.Dir(t.TempDir())
	d, err := Start(ctx, Options{Home: h, Address: "127.0.0.1:0", Version: "test", Env: os.Environ(), Tick: 5 * time.Millisecond, Log: &log})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- d.Serve(ctx) }()
	t.Cleanup(func() {
		d.Stop()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	c, err := api.Dial(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(namedDefinition("beta", "a", "b")), 0o600); err != nil {
		t.Fatal(err)
	}
	// A trigger right after the edit reads the file with force; the change is still logged.
	if _, err := c.Trigger(ctx, "alpha"); err != nil {
		t.Fatal(err)
	}
	waitLog := func(want string, n int) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for strings.Count(log.String(), want) < n {
			if time.Now().After(deadline) {
				t.Fatalf("log has %d of %q, want %d:\n%s", strings.Count(log.String(), want), want, n, log.String())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitLog("pipeline alpha: file changed, read again (it is named beta)", 1)

	if err := os.WriteFile(path, []byte("version: \"1\"\nsteps: [broken: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitLog("pipeline alpha: file changed and does not load", 1)
	// A write can be seen half done, which logs a different message first. Let that
	// settle, then count: many ticks pass with the file still broken and nothing is added.
	time.Sleep(50 * time.Millisecond)
	settled := strings.Count(log.String(), "does not load")
	time.Sleep(200 * time.Millisecond)
	if n := strings.Count(log.String(), "does not load"); n != settled {
		t.Fatalf("a file that stays broken was logged again: %d lines became %d:\n%s", settled, n, log.String())
	}
}

// A run trigger that is the first to see an edit still logs it, however long the tick is.
func TestDaemon_ATriggerThatSeesAnEditFirstLogsIt(t *testing.T) {
	ctx := context.Background()
	var log lockedBuffer
	h := home.Dir(t.TempDir())
	d, err := Start(ctx, Options{Home: h, Address: "127.0.0.1:0", Version: "test", Env: os.Environ(), Tick: time.Hour, Log: &log})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- d.Serve(ctx) }()
	t.Cleanup(func() {
		d.Stop()
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	c, err := api.Dial(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("alpha", "one", "two")), 0o600); err != nil {
		t.Fatal(err)
	}
	if run := mustTrigger(t, c, "alpha"); len(run.Steps) != 2 {
		t.Fatalf("run = %+v, want the edited file", run)
	}
	if !strings.Contains(log.String(), "pipeline alpha: file changed, read again") {
		t.Fatalf("the trigger saw the edit and did not log it:\n%s", log.String())
	}
}

// Registering a file again without --name finds its registration even when the
// name its file now gives is held by another flow (FR-008): no clash, no duplicate.
func TestDaemon_RegisteringAgainIsNoClashWhenTheFilesNewNameIsHeld(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	a := writeDefinition(t, t.TempDir(), "a.yaml", namedDefinition("alpha", "x"))
	b := writeDefinition(t, t.TempDir(), "b.yaml", namedDefinition("beta", "x"))
	for _, path := range []string{a, b} {
		if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(a, []byte(namedDefinition("beta", "x", "y")), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if list := pipelinesOf(t, c); len(list) == 2 && stepIDs(list[0]) == "x,y" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the edit was not seen")
		}
		time.Sleep(5 * time.Millisecond)
	}
	again, err := c.Register(ctx, api.RegisterRequest{Path: a})
	if err != nil || again.Name != "alpha" {
		t.Fatalf("register a again = %+v, %v; want its own registration, not a clash over beta", again, err)
	}
	if list := pipelinesOf(t, c); len(list) != 2 {
		t.Fatalf("pipelines = %d, want 2", len(list))
	}
}

// The list is ordered by the name shown, not the registered one.
func TestDaemon_PipelinesAreListedByTheNameShown(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	a := writeDefinition(t, t.TempDir(), "a.yaml", namedDefinition("alpha", "x"))
	b := writeDefinition(t, t.TempDir(), "b.yaml", namedDefinition("beta", "x"))
	for _, path := range []string{a, b} {
		if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(a, []byte(namedDefinition("zeta", "x", "y")), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		list := pipelinesOf(t, c)
		if len(list) == 2 && list[1].Name == "zeta" {
			if list[0].Name != "beta" {
				t.Fatalf("pipelines = %+v, want beta then zeta", list)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipelines = %+v", list)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A file that fails to load because it could not be read is read again until it can be,
// even though its size and time did not change.
func TestDaemon_AFileThatCouldNotBeReadHealsWithoutAnotherEdit(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file this user may not read")
	}
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	// A new file of another size that nobody may read: stat works, the read fails.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(namedDefinition("alpha", "a", "b")), 0o000); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "unreadable", func(p api.Pipeline) bool { return !p.Valid })
	if err := os.Chmod(path, 0o600); err != nil { // size and time stay as they were
		t.Fatal(err)
	}
	awaitPipeline(t, c, "healed", func(p api.Pipeline) bool { return p.Valid && stepIDs(p) == "a,b" })
}

// A file that fails on its content fails the same way until it changes, so it is not
// parsed again at every check.
func TestDaemon_AFileThatFailsOnItsContentIsNotParsedAgain(t *testing.T) {
	ctx := context.Background()
	d, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "flow.yaml", namedDefinition("alpha", "one"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: \"1\"\nsteps: [broken: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	awaitPipeline(t, c, "invalid", func(p api.Pipeline) bool { return !p.Valid })
	reading := func() *definition {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.defs["alpha"]
	}
	first := reading()
	time.Sleep(150 * time.Millisecond) // about thirty ticks and several requests
	pipelinesOf(t, c)
	if reading() != first {
		t.Fatal("the daemon parsed an unchanged, invalid file again")
	}
}
