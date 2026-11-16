package daemon

import (
	"context"
	"os"
	"strings"
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
	if list, _ := c.Runs(ctx, "", 0); len(list) != 3 {
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
	if list, _ := c.Pipelines(ctx); len(list) != 1 {
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
	if list, _ := c.Runs(ctx, "alpha", 0); len(list) != 3 {
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
