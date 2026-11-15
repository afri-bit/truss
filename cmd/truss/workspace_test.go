package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/home"
)

func showVars() string {
	if runtime.GOOS == "windows" {
		return "echo data=%TRUSS_DATA_DIR% && echo cache=%TRUSS_CACHE_DIR%"
	}
	return "echo data=$TRUSS_DATA_DIR && echo cache=$TRUSS_CACHE_DIR"
}

func TestRun_StepsSeeProjectLocalDataAndCacheFolders(t *testing.T) {
	dir := t.TempDir()
	body := "version: \"1\"\npipeline:\n  name: vars\nsteps:\n  - id: show\n    run: " + showVars() + "\n"
	if err := os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", "pipeline.yaml"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	data := filepath.Join(dir, ".truss", "state", "pipeline", "data")
	for _, want := range []string{"data=" + data, "cache=" + filepath.Join(data, "cache")} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if info, err := os.Stat(filepath.Join(data, "cache")); err != nil || !info.IsDir() {
		t.Fatalf("cache folder not created: %v", err)
	}
}

func TestRun_WorkdirStartsStepsInThatFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "app"), 0o750); err != nil {
		t.Fatal(err)
	}
	pwd := "pwd"
	if runtime.GOOS == "windows" {
		pwd = "cd"
	}
	body := "version: \"1\"\npipeline:\n  name: wd\n  workdir: app\nsteps:\n  - id: where\n    run: " + pwd + "\n"
	if err := os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", "pipeline.yaml"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "step where | ") || !strings.Contains(out, "app") {
		t.Fatalf("step did not run in app:\n%s", out)
	}
}

func TestRun_MissingWorkdirFailsWithADiagnostic(t *testing.T) {
	dir := t.TempDir()
	body := "version: \"1\"\npipeline:\n  name: wd\n  workdir: nope\nsteps:\n  - id: where\n    run: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := invoke(t, dir, []string{"run", "pipeline.yaml"})
	if code != 1 || strings.Contains(out, "exit 0") || !strings.Contains(out, "summary failed") {
		t.Fatalf("exit = %d, out:\n%s", code, out)
	}
}

func TestCacheClear_EmptiesOnlyTheCacheFolder(t *testing.T) {
	s := newSession(t, false)
	h, err := home.Resolve(s.env)
	if err != nil {
		t.Fatal(err)
	}
	cache, other := h.PipelineCache("demo"), h.PipelineCache("other")
	for _, d := range []string{cache, other} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	extra := filepath.Join(h.PipelineData("demo"), "state.txt")
	if err := os.WriteFile(extra, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := s.ok("cache", "clear", "demo"); !strings.Contains(out, "cleared the cache of demo") {
		t.Fatalf("out = %q", out)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("cache still there: %v", err)
	}
	for _, keep := range []string{extra, filepath.Join(other, "f")} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s was removed: %v", keep, err)
		}
	}
	if out := s.ok("cache", "clear", "demo"); !strings.Contains(out, "no cache for demo") {
		t.Fatalf("second clear: %q", out)
	}
	for _, args := range [][]string{{"cache", "clear", "../x"}, {"cache", "clear"}, {"cache"}, {"cache", "wipe", "demo"}} {
		if code, _, _ := s.truss(args...); code != 1 {
			t.Errorf("truss %v exit = %d, want 1", args, code)
		}
	}
}

func TestRun_FailsBeforeAnyStepWhenTheDataFolderCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, ".truss", "state", "pipeline")
	if err := os.MkdirAll(state, 0o750); err != nil {
		t.Fatal(err)
	}
	// A file where the data folder should be.
	if err := os.WriteFile(filepath.Join(state, "data"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := validPipeline("echo ran> marker.txt")
	if err := os.WriteFile(filepath.Join(dir, "pipeline.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := invoke(t, dir, []string{"run", "pipeline.yaml"})
	if code != 1 || !strings.Contains(errOut, "error: prepare state folder:") || strings.Contains(out, "started") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); err == nil {
		t.Fatal("a step ran")
	}
}
