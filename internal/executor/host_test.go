package executor

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecute_Success(t *testing.T) {
	run := `echo "Hello from TRUSS"`
	if runtime.GOOS == "windows" {
		run = `echo Hello from TRUSS`
	}
	res, err := Execute(context.Background(), Request{Run: run, Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Err != nil {
		t.Fatalf("start error = %v", res.Err)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("ExitCode = %v", res.ExitCode)
	}
	if res.Stdout == "" {
		t.Fatal("stdout is empty")
	}
}

func TestExecute_NonZeroExit(t *testing.T) {
	run := "exit 1"
	if runtime.GOOS == "windows" {
		run = "exit /b 1"
	}
	res, err := Execute(context.Background(), Request{Run: run, Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.ExitCode == nil || *res.ExitCode == 0 {
		t.Fatalf("ExitCode = %v, stderr = %s", res.ExitCode, res.Stderr)
	}
}

func TestExecute_MissingProgramHasExitCode(t *testing.T) {
	res, err := Execute(context.Background(), Request{
		Run: "truss-missing-command-xyz",
		Dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Err != nil {
		t.Fatalf("start error = %v", res.Err)
	}
	if res.ExitCode == nil || *res.ExitCode == 0 {
		t.Fatalf("ExitCode = %v, stderr = %s", res.ExitCode, res.Stderr)
	}
	if res.Stderr == "" {
		t.Fatal("expected shell diagnostic on stderr")
	}
}

func TestExecute_CannotStart(t *testing.T) {
	res, err := Execute(context.Background(), Request{
		Run: "echo hi",
		Dir: filepath.Join(t.TempDir(), "missing-dir"),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.ExitCode != nil {
		t.Fatalf("ExitCode = %d, want none", *res.ExitCode)
	}
	if res.Err == nil {
		t.Fatal("expected start error")
	}
}
