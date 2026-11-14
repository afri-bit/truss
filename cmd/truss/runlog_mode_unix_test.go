//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// FR-018: the saved run log has the mode any new file gets (0666 less the
// umask), the same as before the fix, not the owner-only mode of os.CreateTemp.
func TestRun_LogHasTheUsualNewFileMode(t *testing.T) {
	dir := t.TempDir()
	ref, err := os.Create(filepath.Join(t.TempDir(), "reference"))
	if err != nil {
		t.Fatal(err)
	}
	refInfo, err := ref.Stat()
	_ = ref.Close()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := invoke(t, dir, []string{"run", path}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	info, err := os.Stat(filepath.Join(dir, ".truss", "run.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), refInfo.Mode().Perm(); got != want {
		t.Fatalf("run.log mode = %v, want %v (the mode os.Create gives)", got, want)
	}
}
