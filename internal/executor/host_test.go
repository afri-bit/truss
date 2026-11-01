package executor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestShell_ReportsHostShell(t *testing.T) {
	got := Shell()
	if runtime.GOOS == "windows" {
		want := os.Getenv("COMSPEC")
		if want == "" {
			want = "cmd.exe"
		}
		if got != want {
			t.Fatalf("Shell() = %q, want %q", got, want)
		}
		return
	}
	if got != "/bin/sh" {
		t.Fatalf("Shell() = %q, want /bin/sh", got)
	}
}

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

func TestStartWait_Success(t *testing.T) {
	proc, err := Start(context.Background(), Request{Run: helperCmd("print-lines", "2", "hi"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res := proc.Wait(context.Background())
	if res.Err != nil {
		t.Fatalf("Err = %v", res.Err)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("ExitCode = %v", res.ExitCode)
	}
	if res.Stopped {
		t.Fatal("Stopped = true for a Step that finished on its own")
	}
	if !strings.Contains(res.Stdout, "hi 1") || !strings.Contains(res.Stdout, "hi 2") {
		t.Fatalf("Stdout = %q", res.Stdout)
	}
}

func TestStartWait_StreamsToOut(t *testing.T) {
	var out bytes.Buffer
	proc, err := Start(context.Background(), Request{Run: helperCmd("print-lines", "3", "live"), Dir: t.TempDir(), Out: &out})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	proc.Wait(context.Background())
	if strings.Count(out.String(), "live ") != 3 {
		t.Fatalf("Out = %q", out.String())
	}
}

func TestStartWait_NonZeroExitKeepsCode(t *testing.T) {
	proc, err := Start(context.Background(), Request{Run: helperCmd("exit", "7"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	res := proc.Wait(context.Background())
	if res.Err != nil || res.ExitCode == nil || *res.ExitCode != 7 {
		t.Fatalf("result = %+v", res)
	}
	if res.Stopped {
		t.Fatal("Stopped = true for a failed Step")
	}
}

func TestStart_CannotStartReturnsErrorAndNoProcess(t *testing.T) {
	proc, err := Start(context.Background(), Request{
		Run: "echo hi",
		Dir: filepath.Join(t.TempDir(), "missing-dir"),
	})
	if err == nil {
		t.Fatal("Start() error = nil, want start error")
	}
	if proc != nil {
		t.Fatalf("Start() process = %v, want nil", proc)
	}
}

func TestExecute_CannotStartHasNoExitCodeAndNotStopped(t *testing.T) {
	res, err := Execute(context.Background(), Request{
		Run: "echo hi",
		Dir: filepath.Join(t.TempDir(), "missing-dir"),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Err == nil || res.ExitCode != nil || res.Stopped {
		t.Fatalf("result = %+v", res)
	}
}

func TestExecute_SuccessAndFailureAreNotStopped(t *testing.T) {
	for _, code := range []string{"0", "4"} {
		res, err := Execute(context.Background(), Request{Run: helperCmd("exit", code), Dir: t.TempDir()})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if res.Stopped || res.ExitCode == nil || strconv.Itoa(*res.ExitCode) != code {
			t.Fatalf("exit %s: result = %+v", code, res)
		}
	}
}
