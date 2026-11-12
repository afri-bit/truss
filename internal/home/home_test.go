package home

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteJSON_IgnoresAPermissiveLeftoverTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not enforced on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")
	// A loose file where the old writer put its temporary data.
	if err := os.WriteFile(path+".tmp", []byte("old"), 0o644); err != nil { // #nosec G306 -- the point of the test
		t.Fatal(err)
	}
	if err := WriteJSON(path, map[string]string{"token": "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteJSON_DoesNotFollowASymlinkedTempName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "registry.json")
	if err := os.Symlink(victim, path+".tmp"); err != nil {
		t.Skipf("symlink: %v", err)
	}
	if err := WriteJSON(path, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Fatalf("the symlink target was overwritten: %q", b)
	}
}

func TestWriteJSON_LeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clock.json")
	for range 3 {
		if err := WriteJSON(path, map[string]int{"a": 1}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only clock.json", len(entries))
	}
}

func TestEnsure_TightensAnExistingLooseHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory modes are not enforced on Windows")
	}
	dir := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(dir, 0o755); err != nil { // #nosec G301 -- the point of the test
		t.Fatal(err)
	}
	if err := Dir(dir).Ensure(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %v, want 0700", info.Mode().Perm())
	}
}
