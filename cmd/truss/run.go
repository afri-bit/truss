package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/pipeline"
)

// Run resolves the pipeline path, executes it, and writes the result.
// It returns the process exit code.
func Run(ctx context.Context, args []string, cwd string, env []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" || len(args) > 2 {
		fmt.Fprintf(stderr, "error: usage: truss run [<path>]\n")
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
	run, err := engine.Run(ctx, def, cwd, env, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err.Error())
		return 1
	}
	if run.Status != pipeline.StatusSucceeded {
		fmt.Fprintf(stderr, "error: %s\n", failureLine(run))
		return 1
	}
	return 0
}

func failureLine(run pipeline.PipelineRun) string {
	if len(run.Steps) == 0 {
		if run.Error != "" {
			return run.Error
		}
		return run.Name + " failed"
	}
	step := run.Steps[len(run.Steps)-1]
	if step.Error != "" {
		return fmt.Sprintf("%s failed at %s: %s", run.Name, step.ID, step.Error)
	}
	code := 0
	if step.ExitCode != nil {
		code = *step.ExitCode
	}
	return fmt.Sprintf("%s failed at %s (exit %d)", run.Name, step.ID, code)
}
