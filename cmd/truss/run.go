package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/runlog"
)

// runLocal runs a pipeline definition in this process, without the daemon.
// It writes the run log to stdout and to .truss/run.log.
func runLocal(ctx context.Context, args []string, cwd string, env []string, stdout, stderr io.Writer) int {
	path := definitionPath(cwd, args[1:])

	def, err := pipeline.ParseFile(ctx, path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if err := pipeline.Validate(def); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}

	log, tmp, tmpPath, dest, err := openRunLog(cwd, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	dir := cwd
	if abs, absErr := filepath.Abs(cwd); absErr == nil {
		dir = abs
	}
	log.Setup(runlog.OSName(runtime.GOOS), runtime.GOARCH, executor.Shell(), dir)
	run, err := engine.Run(ctx, def, cwd, env, log, engine.WithSecrets(secretLoader(env)))
	if err != nil {
		discardRunLog(tmp, tmpPath)
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if err := ctx.Err(); err != nil {
		// FR-017: an interrupted run never saves a run log.
		discardRunLog(tmp, tmpPath)
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if run.Status != pipeline.StatusSucceeded {
		log.SummaryFailed(len(run.Steps), run.Failure())
		if err := commitRunLog(tmp, tmpPath, dest); err != nil {
			fmt.Fprintf(stderr, "error: %s\n", err.Error())
			return 1
		}
		return 1
	}
	log.SummarySucceeded(len(run.Steps))
	if err := commitRunLog(tmp, tmpPath, dest); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	return 0
}

func openRunLog(cwd string, stdout io.Writer) (*runlog.Log, *os.File, string, string, error) {
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
	return runlog.New(io.MultiWriter(stdout, f), time.Now), f, tmpPath, dest, nil
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
