package daemon

import (
	"context"
	"fmt"
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
	res := scan(root, pipelineFileHook("**"))
	if res.limit != "" || len(res.files) != 2 {
		t.Fatalf("scan = %+v", res)
	}
	if _, ok := res.files["sub/b.txt"]; !ok {
		t.Fatalf("scan = %+v", res)
	}
}

func pipelineFileHook(paths ...string) pipeline.FileHook {
	return pipeline.FileHook{Paths: paths, Debounce: time.Second}
}

// Security review of spec 029: the scan is bounded, names with control
// characters are left out, a linked base is not followed.
func TestHooks_ScanIsBoundedAndCareful(t *testing.T) {
	root := t.TempDir()
	big := filepath.Join(root, "big")
	for i := 0; i < 300; i++ {
		d := filepath.Join(big, fmt.Sprintf("d%03d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 200; j++ {
			if err := os.WriteFile(filepath.Join(d, fmt.Sprintf("f%03d.txt", j)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	res := scan(root, pipelineFileHook("big/**/*.never"))
	if !strings.Contains(res.limit, "looked at more than") {
		t.Fatalf("an unbounded walk: %+v", res.limit)
	}
	if runtime.GOOS != "windows" {
		odd := filepath.Join(root, "inbox")
		if err := os.MkdirAll(odd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(odd, "x\nevil.txt"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(odd, "ok.txt"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		res = scan(root, pipelineFileHook("inbox/"))
		if len(res.files) != 1 || len(res.skipped) == 0 {
			t.Fatalf("control characters: %+v", res)
		}
		if err := os.Symlink(odd, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		res = scan(root, pipelineFileHook("link/"))
		if len(res.files) != 0 || len(res.skipped) != 1 || !strings.Contains(res.skipped[0], "does not follow the link") {
			t.Fatalf("link: %+v", res)
		}
	}
}

// Flows that start each other widely add at most one waiting run each.
func TestHooks_RunHooksDoNotFanOut(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	hooks := "on:\n  run:\n    - {flow: a, status: any}\n    - {flow: b, status: any}\n"
	for _, name := range []string{"a", "b"} {
		body := "version: \"1\"\nflow: {name: " + name + "}\n" + hooks + "steps:\n  - id: s\n    run: echo " + name + "\n"
		if _, err := c.Register(ctx, api.RegisterRequest{Path: writeDefinition(t, dir, name+".yaml", body)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Trigger(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	// The runs stop, and their number grows with the chain limit, not
	// exponentially: each flow has one hook run active and one waiting.
	deadline := time.Now().Add(30 * time.Second)
	last, quiet := -1, 0
	for quiet < 20 {
		as, _ := c.Runs(ctx, "a", 0)
		bs, _ := c.Runs(ctx, "b", 0)
		n := len(as) + len(bs)
		if n == last {
			quiet++
		} else {
			last, quiet = n, 0
		}
		if time.Now().After(deadline) {
			t.Fatalf("the hooks did not stop: %d runs", n)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last > 4*(maxChain+1) {
		t.Fatalf("%d runs: the hooks fanned out", last)
	}
}

// Resuming takes a new baseline: changes made while paused start nothing,
// changes after resuming do (review of spec 029).
func TestHooks_ResumeCountsFromNow(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	watched := mkdirs(t, filepath.Join(dir, "w"))
	flow := writeDefinition(t, dir, "r.yaml", "version: \"1\"\nflow: {name: r}\non:\n  file: {paths: [w/], debounce: 1s}\nsteps:\n  - id: a\n    run: echo a\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetPaused(ctx, "r", true); err != nil {
		t.Fatal(err)
	}
	writeDefinition(t, watched, "while-paused", "1")
	time.Sleep(200 * time.Millisecond)
	if _, err := c.SetPaused(ctx, "r", false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if list, _ := c.Runs(ctx, "r", 0); len(list) != 0 {
		t.Fatalf("a change made while paused started a run: %+v", list)
	}
	writeDefinition(t, watched, "after", "2")
	awaitRuns(t, c, "r", 1)
}

// An on.run match while the flow's hook run is still active waits and starts
// after it (delta review of spec 029); a renamed flow is matched by the name
// in its file too.
func TestHooks_ABusyOnRunWaitsAndNamesMatch(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	slow := "sleep 1"
	if runtime.GOOS == "windows" {
		slow = "ping -n 2 127.0.0.1 >nul"
	}
	for name, body := range map[string]string{
		// registered as "build", shown as "ci-build"
		"build.yaml":  "version: \"1\"\nflow: {name: ci-build}\nsteps:\n  - id: a\n    run: echo built\n",
		"deploy.yaml": "version: \"1\"\nflow: {name: deploy}\non:\n  run: {flow: ci-build}\nsteps:\n  - id: a\n    run: " + slow + "\n",
	} {
		reg := strings.TrimSuffix(name, ".yaml")
		if _, err := c.Register(ctx, api.RegisterRequest{Path: writeDefinition(t, dir, name, body), Name: reg}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ { // three builds while the first deploy is still running
		if _, err := c.Trigger(ctx, "build"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// One deploy for the first build, one waiting run for the next two.
	awaitRuns(t, c, "deploy", 2)
	time.Sleep(2 * time.Second)
	list, _ := c.Runs(ctx, "deploy", 0)
	if len(list) != 2 {
		t.Fatalf("deploys = %d, want 2", len(list))
	}
	for _, r := range list {
		if r.Trigger != api.TriggerRun || r.Status != api.RunSucceeded {
			t.Fatalf("deploy = %+v", r)
		}
	}
}
