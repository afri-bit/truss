package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A checkout can carry symbolic links inside .truss (git stores links). Writing
// the run log must never write through such a link into a file elsewhere.

func plantLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		// Windows without Developer Mode or the privilege cannot create links,
		// and then a checkout cannot plant one either.
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
}

func TestRun_PlantedTempLinkIsNotFollowed(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim.txt")
	const keep = "do not touch\n"
	if err := os.WriteFile(victim, []byte(keep), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".truss"), 0o755); err != nil {
		t.Fatal(err)
	}
	plantLink(t, victim, filepath.Join(dir, ".truss", "run.log.tmp"))
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != keep {
		t.Fatalf("file behind the planted link changed: %q, %v", got, err)
	}
	if saved := readRunLog(t, dir); saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
}

func TestRun_PlantedLogLinkIsReplacedNotFollowed(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim.txt")
	const keep = "do not touch\n"
	if err := os.WriteFile(victim, []byte(keep), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".truss"), 0o755); err != nil {
		t.Fatal(err)
	}
	plantLink(t, victim, filepath.Join(dir, ".truss", "run.log"))
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := invoke(t, dir, []string{"run", path})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != keep {
		t.Fatalf("file behind the planted link changed: %q, %v", got, err)
	}
	info, err := os.Lstat(filepath.Join(dir, ".truss", "run.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("run.log is %v, want a regular file", info.Mode())
	}
	if saved := readRunLog(t, dir); saved != out {
		t.Fatalf("saved log =\n%s\nstdout =\n%s", saved, out)
	}
}

func TestRun_LeavesNoTemporaryLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := invoke(t, dir, []string{"run", path}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".truss"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temporary log left behind: %s", e.Name())
		}
	}
}
