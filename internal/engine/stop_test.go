//go:build unix

package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/pipeline"
)

// Spec 028: a step that times out is asked to stop first and may clean up.
// The timeout is generous so the shell has its trap before it fires.
func TestRun_ATimedOutStepGetsSIGTERMAndMayCleanUp(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleaned")
	run := "trap 'echo cleaned > \"" + marker + "\"; exit 0' TERM; while :; do sleep 0.05; done"
	p := pipeline.Pipeline{Version: "1", Name: "stop", Steps: []pipeline.Step{{ID: "a", Run: run, Timeout: 2 * time.Second}}}
	got, err := Run(context.Background(), p, t.TempDir(), os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// It cleaned up, and exiting 0 after the stop does not make it succeed.
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the step did not get to clean up: %v", err)
	}
	if got.Status != pipeline.StatusFailed || !strings.Contains(got.Steps[0].Error, "timed out") {
		t.Fatalf("run = %+v", got)
	}
}

// A step that ignores the request is killed after kill_timeout. The run is
// cancelled only once the shell has its trap, and timed from then.
func TestRun_AStepThatIgnoresTheStopIsKilledAfterKillTimeout(t *testing.T) {
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	run := "trap '' TERM; touch \"" + started + "\"; while :; do sleep 0.05; done"
	p := pipeline.Pipeline{Version: "1", Name: "stop", Steps: []pipeline.Step{
		{ID: "a", Run: run, KillTimeout: 500 * time.Millisecond}}}
	ctx, cancel := context.WithCancel(context.Background())
	var asked time.Time
	go func() {
		for i := 0; i < 1000; i++ {
			if _, err := os.Stat(started); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		asked = time.Now()
		cancel()
	}()
	_, _ = Run(ctx, p, t.TempDir(), os.Environ(), nil)
	if took := time.Since(asked); took < 450*time.Millisecond || took > 10*time.Second {
		t.Fatalf("ended %s after the stop, want about kill_timeout", took)
	}
}

// Cancelling a run also asks first.
func TestRun_CancellingARunLetsStepsCleanUp(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "cleaned")
	run := "trap 'echo cleaned > \"" + marker + "\"; exit 1' TERM; touch \"" + filepath.Join(dir, "started") + "\"; while :; do sleep 0.05; done"
	p := pipeline.Pipeline{Version: "1", Name: "stop", Steps: []pipeline.Step{{ID: "a", Run: run}}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for i := 0; i < 200; i++ {
			if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()
	_, _ = Run(ctx, p, t.TempDir(), os.Environ(), nil)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the step did not get to clean up on cancel: %v", err)
	}
}
