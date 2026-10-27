package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
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

func TestRun_TwoStepsPrintSucceededSummary(t *testing.T) {
	dir := t.TempDir()
	second := "echo second"
	body := "version: \"1\"\npipeline:\n  name: hello-world\nsteps:\n  - id: hello\n    run: " + echoHello() + "\n  - id: where\n    run: " + second + "\n"
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	assertSuccessReport(t, out, "hello")
	hello := strings.Index(out, "step hello started")
	where := strings.Index(out, "step where started")
	if hello < 0 || where < hello {
		t.Fatalf("steps out of order:\n%s", out)
	}
	if !strings.Contains(out, "step where |") || !strings.Contains(out, "summary succeeded steps 2") {
		t.Fatalf("summary missing:\n%s", out)
	}
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
	if !strings.Contains(out, "step first started") || !strings.Contains(out, "step second started") {
		t.Fatalf("report = %s", out)
	}
	if strings.Contains(out, "step third") {
		t.Fatalf("third step reported:\n%s", out)
	}
	if !strings.Contains(out, "summary failed steps 2") || !strings.Contains(out, "error fail-fast failed at second (exit 1)") {
		t.Fatalf("report = %s", out)
	}
	if strings.Contains(errOut, "failed at second") {
		t.Fatalf("stderr duplicates the run log: %s", errOut)
	}
	if saved := readRunLog(t, dir); saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("third step ran: %v", err)
	}
}

func TestRun_RejectedDefinitionLeavesLogUnchanged(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, ".truss", "run.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const previous = "previous log\n"
	if err := os.WriteFile(logPath, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		":\n: not yaml\n",
		"version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    run: \"\"\n",
	}
	for _, body := range cases {
		path := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		code, out, errOut := invoke(t, dir, []string{"run", path})
		if code != 1 {
			t.Fatalf("exit = %d, stderr = %s", code, errOut)
		}
		if !strings.Contains(errOut, "error:") {
			t.Fatalf("stderr = %q", errOut)
		}
		if strings.Contains(out, "step ") {
			t.Fatalf("stdout ran a step:\n%s", out)
		}
		got, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != previous {
			t.Fatalf("log changed to %q", got)
		}
	}
}

func TestRun_TrussFileStartsNoStep(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran.txt")
	run := "echo x > '" + marker + "'"
	if runtime.GOOS == "windows" {
		run = "echo x> \"" + marker + "\""
	}
	const previous = "previous log\n"
	if err := os.WriteFile(filepath.Join(dir, ".truss"), []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(run)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(errOut, "error:") {
		t.Fatalf("stderr = %q", errOut)
	}
	if strings.Contains(out, "step ") {
		t.Fatalf("stdout ran a step:\n%s", out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("step ran: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".truss"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != previous {
		t.Fatalf(".truss changed to %q", got)
	}
}

func TestRun_SavesLogMatchingStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	saved := readRunLog(t, dir)
	if saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
}

func TestRun_SecondRunReplacesLog(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(first, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "version: \"1\"\npipeline:\n  name: second-run\nsteps:\n  - id: later\n    run: echo later\n"
	if err := os.WriteFile(second, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := invoke(t, dir, []string{"run", first}); code != 0 {
		t.Fatalf("first exit = %d, stderr = %s", code, errOut)
	}
	code, out, errOut := invoke(t, dir, []string{"run", second})
	if code != 0 {
		t.Fatalf("second exit = %d, stderr = %s", code, errOut)
	}
	saved := readRunLog(t, dir)
	if saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
	if strings.Contains(saved, "step hello started") || !strings.Contains(saved, "step later started") {
		t.Fatalf("log was not replaced:\n%s", saved)
	}
}

func TestRun_LogStaysInWorkingDirectory(t *testing.T) {
	work := t.TempDir()
	other := t.TempDir()
	path := filepath.Join(other, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, work, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	saved := readRunLog(t, work)
	if saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
	if _, err := os.Stat(filepath.Join(other, ".truss", "run.log")); !os.IsNotExist(err) {
		t.Fatalf("log written beside the definition: %v", err)
	}
}

func readRunLog(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".truss", "run.log"))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	return string(data)
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

var timestampPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} `)

func assertSuccessReport(t *testing.T, out, stepID string) {
	t.Helper()
	if strings.Contains(out, "["+stepID+"]") {
		t.Fatalf("old transcript:\n%s", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !timestampPrefix.MatchString(line) {
			t.Fatalf("line missing timestamp %q\n%s", line, out)
		}
	}
	setup := strings.Index(out, "setup environment ready")
	stepOut := strings.Index(out, "step "+stepID+" |")
	if setup < 0 || stepOut < 0 || setup > stepOut {
		t.Fatalf("setup is not before step output:\n%s", out)
	}
	for _, want := range []string{
		"setup os ",
		"setup arch ",
		"setup shell ",
		"setup directory ",
		"step " + stepID + " started",
		"step " + stepID + " finished duration ",
		"exit 0",
		"Hello from TRUSS",
		"summary succeeded steps ",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
	for _, noise := range []string{"pipeline:", "status:", "exit_code:", "stderr:", "output:", "PATH="} {
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

const wantUsage = "usage: truss run [<path>]\n       truss help\n       truss version\n"

func TestHelp_PrintsUsageWithoutRunLog(t *testing.T) {
	cases := [][]string{nil, {"help"}, {"--help"}}
	for _, args := range cases {
		dir := t.TempDir()
		code, out, errOut := invoke(t, dir, args)
		if code != 0 {
			t.Fatalf("args %q exit = %d, stderr = %s", args, code, errOut)
		}
		if out != wantUsage {
			t.Fatalf("args %q stdout = %q", args, out)
		}
		if errOut != "" {
			t.Fatalf("args %q stderr = %q", args, errOut)
		}
		assertNoRunLog(t, dir)
	}
}

func TestVersion_PrintsGoModVersion(t *testing.T) {
	want := versionFromGoMod(t)
	if Version != want {
		t.Fatalf("Version = %q, go.mod // version: = %q", Version, want)
	}
	for _, args := range [][]string{{"version"}, {"--version"}} {
		dir := t.TempDir()
		code, out, errOut := invoke(t, dir, args)
		if code != 0 {
			t.Fatalf("args %q exit = %d, stderr = %s", args, code, errOut)
		}
		if out != want+"\n" {
			t.Fatalf("args %q stdout = %q", args, out)
		}
		if errOut != "" {
			t.Fatalf("args %q stderr = %q", args, errOut)
		}
		assertNoRunLog(t, dir)
	}
}

func TestUnknownInvocation_PrintsUsageOnStderr(t *testing.T) {
	cases := [][]string{
		{"nope"},
		{"help", "extra"},
		{"version", "extra"},
		{"--help", "extra"},
		{"--version", "extra"},
		{"run", "a", "b"},
	}
	for _, args := range cases {
		dir := t.TempDir()
		code, out, errOut := invoke(t, dir, args)
		if code != 1 {
			t.Fatalf("args %q exit = %d", args, code)
		}
		if errOut != wantUsage {
			t.Fatalf("args %q stderr = %q", args, errOut)
		}
		if out != "" {
			t.Fatalf("args %q stdout = %q", args, out)
		}
		assertNoRunLog(t, dir)
	}
}

func TestRun_HelpFlagIsAPath(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := invoke(t, dir, []string{"run", "--help"})
	if code != 1 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(errOut, "error:") {
		t.Fatalf("stderr = %q", errOut)
	}
	if strings.Contains(out, "usage:") || strings.Contains(errOut, "truss version") {
		t.Fatalf("stdout = %q, stderr = %q", out, errOut)
	}
	assertNoRunLog(t, dir)
}

func assertNoRunLog(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".truss", "run.log")); !os.IsNotExist(err) {
		t.Fatalf("run log exists: %v", err)
	}
}

func versionFromGoMod(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				const prefix = "// version:"
				if strings.HasPrefix(line, prefix) {
					return strings.TrimSpace(strings.TrimPrefix(line, prefix))
				}
			}
			t.Fatal("go.mod has no // version: line")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
