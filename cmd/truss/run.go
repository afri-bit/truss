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

// usageText is the CLI usage printed for help and for unknown invocations.
const usageText = "usage: truss run [<path>]\n       truss help\n       truss version\n"

// Run dispatches one CLI invocation and returns the process exit code.
// Help and version write to stdout and do not start a run.
// A pipeline run writes the run log to stdout and to .truss/run.log.
func Run(ctx context.Context, args []string, cwd string, env []string, stdout, stderr io.Writer) int {
	switch {
	case isHelp(args):
		fmt.Fprint(stdout, usageText)
		return 0
	case isVersion(args):
		fmt.Fprintln(stdout, Version)
		return 0
	case !isRun(args):
		fmt.Fprint(stderr, usageText)
		return 1
	}
	path := filepath.Join(cwd, "truss.yaml")
	if len(args) == 2 {
		path = args[1]
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
	}

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
	log.Setup(hostOS(runtime.GOOS), runtime.GOARCH, executor.Shell(), dir)
	run, err := engine.Run(ctx, def, cwd, env, log)
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
		log.SummaryFailed(len(run.Steps), failureLine(run))
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

func isHelp(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help"))
}

func isVersion(args []string) bool {
	return len(args) == 1 && (args[0] == "version" || args[0] == "--version")
}

func isRun(args []string) bool {
	return len(args) >= 1 && len(args) <= 2 && args[0] == "run"
}

func hostOS(goos string) string {
	switch goos {
	case "darwin":
		return "macOS"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return goos
	}
}

func failureLine(run pipeline.PipelineRun) string {
	if len(run.Steps) == 0 {
		if run.Error != "" {
			return run.Error
		}
		return run.Name + " failed"
	}
	step := run.Steps[len(run.Steps)-1]
	for _, s := range run.Steps {
		if s.ID == run.FailedStep {
			step = s
			break
		}
	}
	if step.Error != "" {
		return fmt.Sprintf("%s failed at %s: %s", run.Name, step.ID, step.Error)
	}
	code := 0
	if step.ExitCode != nil {
		code = *step.ExitCode
	}
	return fmt.Sprintf("%s failed at %s (exit %d)", run.Name, step.ID, code)
}
