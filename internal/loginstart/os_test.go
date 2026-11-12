package loginstart

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOSFiles_WriteReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "file")
	var f OSFiles
	if err := f.Write(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Read(path); string(got) != "two" {
		t.Errorf("content = %q", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
	if err := f.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := f.Remove(path); err != nil {
		t.Errorf("removing a missing file: %v", err)
	}
}

func TestOSFiles_WriteReplacesASymlinkInsteadOfFollowingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := (OSFiles{}).Write(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "keep" {
		t.Errorf("the link's target was overwritten: %q", got)
	}
}
