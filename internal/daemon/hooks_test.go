package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/pipeline"
)

func catChanged() string {
	if runtime.GOOS == "windows" {
		return "type \"%TRUSS_CHANGED_FILES%\" & echo trigger %TRUSS_TRIGGER%"
	}
	return "cat \"$TRUSS_CHANGED_FILES\"; echo trigger $TRUSS_TRIGGER"
}

// Spec 029: a change under a watched folder starts one run after the
// debounce, with the changed paths; ignored paths and the baseline do not.
func TestHooks_FileChangesStartOneRun(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	root := filepath.Join(t.TempDir(), "proj")
	writeDefinition(t, mkdirs(t, root), "truss.yaml", "version: \"1\"\nproject:\n  name: proj\n")
	inbox := mkdirs(t, filepath.Join(root, "inbox"))
	writeDefinition(t, inbox, "before.txt", "already here") // the baseline, not a change
	flow := writeDefinition(t, mkdirs(t, filepath.Join(root, ".truss", "flows")), "sort.yaml",
		"version: \"1\"\nflow: {name: sort}\non:\n  file:\n    paths: [inbox/]\n    ignore: [\"**/*.tmp\"]\n    debounce: 1s\n"+
			"steps:\n  - id: show\n    run: "+catChanged()+"\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // let the daemon take its baseline
	writeDefinition(t, inbox, "a.jpg", "1")
	writeDefinition(t, inbox, "b.jpg", "2")
	writeDefinition(t, inbox, "skip.tmp", "3")
	awaitRuns(t, c, "sort", 1)
	time.Sleep(1500 * time.Millisecond) // no second run for the same burst
	list := awaitRuns(t, c, "sort", 1)
	if len(list) != 1 {
		t.Fatalf("runs = %+v, want one", list)
	}
	run := list[0]
	if run.Trigger != api.TriggerFile || run.Status != api.RunSucceeded {
		t.Fatalf("run = %+v", run)
	}
	data, _, _, err := c.Log(ctx, run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if !strings.Contains(log, "inbox/a.jpg") || !strings.Contains(log, "inbox/b.jpg") || strings.Contains(log, "skip.tmp") ||
		strings.Contains(log, "before.txt") || !strings.Contains(log, "trigger file") {
		t.Fatalf("log:\n%s", log)
	}
}

// A paused flow does not start on changes.
func TestHooks_APausedFlowIgnoresChanges(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	watched := mkdirs(t, filepath.Join(dir, "w"))
	flow := writeDefinition(t, dir, "p.yaml", "version: \"1\"\nflow: {name: p}\non:\n  file: {paths: [w/], debounce: 1s}\nsteps:\n  - id: a\n    run: echo a\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetPaused(ctx, "p", true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	writeDefinition(t, watched, "x", "1")
	time.Sleep(1500 * time.Millisecond)
	if list, _ := c.Runs(ctx, "p", 0); len(list) != 0 {
		t.Fatalf("a paused flow ran: %+v", list)
	}
}

// on.run starts a flow when another one's run ends as asked; a loop of flows
// that start each other stops at the chain limit.
func TestHooks_RunHooksFollowAndStopALoop(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	for name, body := range map[string]string{
		"build.yaml":  "version: \"1\"\nflow: {name: build}\nsteps:\n  - id: a\n    run: echo built\n",
		"deploy.yaml": "version: \"1\"\nflow: {name: deploy}\non:\n  run: {flow: build}\nsteps:\n  - id: a\n    run: echo deployed\n",
		"alert.yaml":  "version: \"1\"\nflow: {name: alert}\non:\n  run: {flow: build, status: failed}\nsteps:\n  - id: a\n    run: echo alert\n",
		"ping.yaml":   "version: \"1\"\nflow: {name: ping}\non:\n  run: {flow: pong}\nsteps:\n  - id: a\n    run: echo ping\n",
		"pong.yaml":   "version: \"1\"\nflow: {name: pong}\non:\n  run: {flow: ping}\nsteps:\n  - id: a\n    run: echo pong\n",
	} {
		if _, err := c.Register(ctx, api.RegisterRequest{Path: writeDefinition(t, dir, name, body)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Trigger(ctx, "build"); err != nil {
		t.Fatal(err)
	}
	if list := awaitRuns(t, c, "deploy", 1); list[0].Trigger != api.TriggerRun {
		t.Fatalf("deploy = %+v", list)
	}
	time.Sleep(300 * time.Millisecond)
	if list, _ := c.Runs(ctx, "alert", 0); len(list) != 0 {
		t.Fatalf("alert ran after a success: %+v", list)
	}
	if _, err := c.Trigger(ctx, "ping"); err != nil {
		t.Fatal(err)
	}
	// ping (manual, chain 0) -> pong 1 -> ping 2 -> ... stops after maxChain.
	deadline := time.Now().Add(20 * time.Second)
	for {
		pings, _ := c.Runs(ctx, "ping", 0)
		pongs, _ := c.Runs(ctx, "pong", 0)
		if len(pings)+len(pongs) >= maxChain+1 {
			time.Sleep(500 * time.Millisecond)
			pings, _ = c.Runs(ctx, "ping", 0)
			pongs, _ = c.Runs(ctx, "pong", 0)
			if n := len(pings) + len(pongs); n != maxChain+1 {
				t.Fatalf("%d runs in the loop, want %d", n, maxChain+1)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the loop stopped early: %d ping, %d pong", len(pings), len(pongs))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHooks_ScanSkipsStateAndGit(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"a.txt", ".git/HEAD", ".truss/state/x/run.log", "sub/b.txt"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	snap, full := scan(root, pipelineFileHook("**"))
	if full || len(snap) != 2 {
		t.Fatalf("scan = %v", snap)
	}
	if _, ok := snap["sub/b.txt"]; !ok {
		t.Fatalf("scan = %v", snap)
	}
}

func pipelineFileHook(paths ...string) pipeline.FileHook {
	return pipeline.FileHook{Paths: paths, Debounce: time.Second}
}
