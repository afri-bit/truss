package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/runlog"
)

// runLocal runs a pipeline definition in this process, without the daemon.
// It writes the run log to stdout and to .truss/run.log. With track set it
// also reports the run to the daemon, which must be running (spec 010).
func runLocal(ctx context.Context, pos []string, track bool, cwd string, env []string, stdout, stderr io.Writer) int {
	path := definitionPath(cwd, pos)

	def, err := pipeline.ParseFile(ctx, path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if err := pipeline.Validate(def); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	var tr *tracker
	if track {
		tr, err = startTracking(ctx, def, env, stderr, cancelRun)
		if err != nil {
			fmt.Fprintf(stderr, "error: %s\n", err.Error())
			return 1
		}
	}
	code, status, reason := execute(ctx, runCtx, def, cwd, env, stdout, stderr, tr)
	if tr != nil {
		tr.finish(status, reason)
	}
	return code
}

// startTracking opens the run on the daemon of this invocation's TRUSS home.
func startTracking(ctx context.Context, def pipeline.Pipeline, env []string, stderr io.Writer, cancelRun context.CancelFunc) (*tracker, error) {
	plan, err := pipeline.Plan(def)
	if err != nil {
		return nil, err
	}
	h, err := home.Resolve(env)
	if err != nil {
		return nil, err
	}
	c, err := api.Dial(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("--track: %w", err)
	}
	req := api.CreateRunRequest{Pipeline: plan.Name}
	for _, s := range plan.Graph {
		req.Steps = append(req.Steps, api.PlanStep{ID: s.ID, Needs: s.Needs})
	}
	tr, err := newTracker(ctx, c, req, stderr, cancelRun)
	if err != nil {
		return nil, fmt.Errorf("--track: %w", err)
	}
	fmt.Fprintf(stderr, "tracking as run %s (truss logs %s)\n", tr.id, tr.id)
	return tr, nil
}

// execute runs the pipeline and returns the exit code and how the run ended
// (an api run status and reason) for the tracker. ctx is the caller's;
// runCtx also ends when the daemon cancels a tracked run.
func execute(ctx, runCtx context.Context, def pipeline.Pipeline, cwd string, env []string, stdout, stderr io.Writer, tr *tracker) (int, string, string) {
	var extra io.Writer
	if tr != nil {
		extra = tr
	}
	log, tmp, tmpPath, dest, err := openRunLog(cwd, stdout, extra)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1, api.RunFailed, err.Error()
	}
	dir := cwd
	if abs, absErr := filepath.Abs(cwd); absErr == nil {
		dir = abs
	}
	log.Setup(runlog.OSName(runtime.GOOS), runtime.GOARCH, executor.Shell(), dir)
	var events engine.StepEvents = log
	if tr != nil {
		events = newTrackedEvents(log, tr, runCtx)
	}
	run, err := engine.Run(runCtx, def, cwd, env, events, engine.WithSecrets(secretLoader(env)))
	if err != nil {
		discardRunLog(tmp, tmpPath)
		return interrupted(ctx, tr, err, stderr)
	}
	if err := ctx.Err(); err != nil {
		// A cancel from the daemon that arrives after the run ended does not
		// change its outcome. FR-017: an interrupted run never saves a run log.
		discardRunLog(tmp, tmpPath)
		return interrupted(ctx, tr, err, stderr)
	}
	if run.Status != pipeline.StatusSucceeded {
		log.SummaryFailed(len(run.Steps), run.Failure())
		if err := commitRunLog(tmp, tmpPath, dest); err != nil {
			fmt.Fprintf(stderr, "error: %s\n", err.Error())
		}
		return 1, api.RunFailed, run.Failure()
	}
	log.SummarySucceeded(len(run.Steps))
	if err := commitRunLog(tmp, tmpPath, dest); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1, api.RunFailed, err.Error()
	}
	return 0, api.RunSucceeded, ""
}

// interrupted reports a run that ended early: the caller interrupted it, the
// daemon cancelled it, or the engine refused it.
func interrupted(ctx context.Context, tr *tracker, err error, stderr io.Writer) (int, string, string) {
	switch {
	case tr != nil && tr.daemonCancelled() && ctx.Err() == nil:
		fmt.Fprintln(stderr, "error: cancelled from the daemon")
		return 1, api.RunCancelled, "cancelled from the daemon"
	case ctx.Err() != nil:
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1, api.RunCancelled, "interrupted"
	}
	fmt.Fprintf(stderr, "error: %s\n", err.Error())
	return 1, api.RunFailed, err.Error()
}

func openRunLog(cwd string, stdout, extra io.Writer) (*runlog.Log, *os.File, string, string, error) {
	dir := filepath.Join(cwd, ".truss")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, "", "", fmt.Errorf("prepare run log: %w", err)
	}
	tmpPath := filepath.Join(dir, "run.log.tmp")
	dest := filepath.Join(dir, "run.log")
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("prepare run log: %w", err)
	}
	out := io.MultiWriter(stdout, f)
	if extra != nil {
		out = io.MultiWriter(stdout, f, extra)
	}
	return runlog.New(out, time.Now), f, tmpPath, dest, nil
}

func commitRunLog(tmp *os.File, tmpPath, dest string) error {
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close run log: %w", err)
	}
	if err := replaceFile(tmpPath, dest); err != nil {
		return fmt.Errorf("replace run log: %w", err)
	}
	return nil
}

// replaceFile moves tmpPath to dest.
// When the host cannot replace an existing dest, the previous file is restored
// if the move fails.
func replaceFile(tmpPath, dest string) error {
	err := os.Rename(tmpPath, dest)
	if err == nil {
		return nil
	}
	if _, statErr := os.Stat(dest); statErr != nil {
		return err
	}
	backup := dest + ".bak"
	_ = os.Remove(backup)
	if renameErr := os.Rename(dest, backup); renameErr != nil {
		return err
	}
	if renameErr := os.Rename(tmpPath, dest); renameErr != nil {
		_ = os.Rename(backup, dest)
		return renameErr
	}
	_ = os.Remove(backup)
	return nil
}

func discardRunLog(tmp *os.File, tmpPath string) {
	_ = tmp.Close()
	_ = os.Remove(tmpPath)
}
