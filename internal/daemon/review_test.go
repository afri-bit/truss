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

// A manual trigger must not move the schedule clock: a catch-up that is due
// but not yet ticked still happens (spec 006 FR-019, FR-020).
func TestDaemon_ManualTriggerKeepsAPendingCatchUp(t *testing.T) {
	h := seed(t, "run")
	clock := &fakeClock{now: today(11, 0, 0)}
	d, err := Start(context.Background(), Options{Home: h, Address: "127.0.0.1:0", Env: os.Environ(), Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.listener.Close(); d.runs.Close() })

	if _, err := d.submit("nightly", api.TriggerManual, time.Time{}); err != nil {
		t.Fatal(err)
	}
	d.tick()
	catchUps := 0
	for _, r := range d.runs.List("nightly", 0) {
		if r.Trigger == api.TriggerCatchUp {
			catchUps++
		}
	}
	if catchUps != 1 {
		t.Fatalf("catch-up runs = %d, want 1; runs = %+v", catchUps, d.runs.List("nightly", 0))
	}
}

// FR-021: a schedule whose file is invalid when it fires records a failed run.
func TestDaemon_ScheduleWithInvalidFileAtFireTimeRecordsAFailedRun(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: today(8, 59, 50)}
	_, c := boot(t, home.Dir(t.TempDir()), clock)
	path := writeDefinition(t, t.TempDir(), "nightly.yaml", scheduled("skip"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: \"2\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // let a tick see the broken file
	clock.Set(today(9, 0, 2))
	list := awaitRuns(t, c, "nightly", 1)
	if len(list) != 1 || list[0].Status != api.RunFailed {
		t.Fatalf("runs = %+v, want one failed run", list)
	}
}

func TestDaemon_RefusesToRegisterWhatIsNotARegularFileOrIsHuge(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	if _, err := c.Register(ctx, api.RegisterRequest{Path: dir, Name: "dir"}); err == nil {
		t.Fatal("a directory was registered")
	}
	big := writeDefinition(t, t.TempDir(), "big.yaml", "version: \"1\"\n# "+strings.Repeat("x", maxDefinitionBytes+1)+"\n")
	_, err := c.Register(ctx, api.RegisterRequest{Path: big, Name: "big"})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v, want a too-large refusal", err)
	}
}
