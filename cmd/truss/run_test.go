package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRun_ExplicitPathSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	assertSuccessReport(t, out, "hello")
}

func TestRun_OtherFilenameSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	assertSuccessReport(t, out, "hello")
}

func TestRun_InvalidFileDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	body := "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: \"\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(errOut, "error:") {
		t.Fatalf("stderr = %q", errOut)
	}
	if strings.Contains(out, "step:") {
		t.Fatalf("stdout ran a step:\n%s", out)
	}
}

func TestRun_MissingDefaultFile(t *testing.T) {
	code, out, errOut := invoke(t, t.TempDir(), []string{"run"})
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(errOut, "error:") {
		t.Fatalf("stderr = %q", errOut)
	}
	if strings.Contains(out, "step:") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestRun_FailedStepStopsPipeline(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "third.txt")
	third := "echo third > '" + marker + "'"
	second := "exit 1"
	if runtime.GOOS == "windows" {
		third = "echo third> \"" + marker + "\""
		second = "exit /b 1"
	}
	body := "version: \"1\"\npipeline:\n  name: fail-fast\nsteps:\n  - id: first\n    run: " + echoHello() + "\n  - id: second\n    run: " + second + "\n  - id: third\n    run: " + third + "\n"
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "[first]") || !strings.Contains(out, "[second]") {
		t.Fatalf("report = %s", out)
	}
	if strings.Contains(out, "[third]") {
		t.Fatalf("third step reported:\n%s", out)
	}
	if !strings.Contains(errOut, "failed at second") || !strings.Contains(errOut, "exit 1") {
		t.Fatalf("stderr = %s", errOut)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("third step ran: %v", err)
	}
}

func TestRun_DefaultTrussYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "truss.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	assertSuccessReport(t, out, "hello")
}

func invoke(t *testing.T, cwd string, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), args, cwd, os.Environ(), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func assertSuccessReport(t *testing.T, out, stepID string) {
	t.Helper()
	for _, want := range []string{"[" + stepID + "]", "Hello from TRUSS"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
	for _, noise := range []string{"pipeline:", "status:", "exit_code:", "stderr:", "output:"} {
		if strings.Contains(out, noise) {
			t.Fatalf("report includes %q:\n%s", noise, out)
		}
	}
}

func validPipeline(run string) string {
	return "version: \"1\"\n\npipeline:\n  name: hello-world\n\nsteps:\n  - id: hello\n    run: " + run + "\n"
}

func echoHello() string {
	if runtime.GOOS == "windows" {
		return "echo Hello from TRUSS"
	}
	return "echo \"Hello from TRUSS\""
}
