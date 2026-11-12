package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// canary is a made-up value, never a real credential.
const canary = "canary-value-0001"

func newStore(t *testing.T) *Store {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "home", "secrets.json"))
}

func TestStore_SetLoadNamesRemove(t *testing.T) {
	s := newStore(t)
	if got, err := s.Load(); err != nil || len(got) != 0 {
		t.Fatalf("empty store: %v, %v", got, err)
	}
	for _, kv := range [][2]string{{"B_TOKEN", "b-" + canary}, {"A_TOKEN", "a-" + canary}, {"B_TOKEN", "b2-" + canary}} {
		if err := s.Set(kv[0], kv[1]); err != nil {
			t.Fatalf("Set %s: %v", kv[0], err)
		}
	}
	names, err := s.Names()
	if err != nil || strings.Join(names, ",") != "A_TOKEN,B_TOKEN" {
		t.Fatalf("Names = %v, %v", names, err)
	}
	all, _ := s.Load()
	if all["B_TOKEN"] != "b2-"+canary {
		t.Errorf("B_TOKEN = %q, want the replacement", all["B_TOKEN"])
	}
	if err := s.Remove("A_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("A_TOKEN"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Remove = %v, want ErrNotFound", err)
	}
}

func TestStore_RejectsBadInput(t *testing.T) {
	s := newStore(t)
	for _, name := range []string{"", "1ABC", "A-B", "A B", "A=B"} {
		if err := s.Set(name, canary); err == nil {
			t.Errorf("Set(%q) succeeded", name)
		}
	}
	err := s.Set("OK", "")
	if err == nil {
		t.Error("empty value accepted")
	}
	if _, statErr := os.Stat(s.Path()); statErr == nil {
		t.Error("a rejected Set created the file")
	}
}

func TestStore_FileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not exist on Windows")
	}
	s := newStore(t)
	if err := s.Set("TOKEN", canary); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", info.Mode().Perm())
	}
	dir, _ := os.Stat(filepath.Dir(s.Path()))
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("directory mode = %v, want 0700", dir.Mode().Perm())
	}
}

func TestStore_ReplacingLeavesNoTemporaryFile(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 3; i++ {
		if err := s.Set("TOKEN", canary+string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path()))
	if err != nil || len(entries) != 1 || entries[0].Name() != "secrets.json" {
		t.Fatalf("directory = %v, %v", entries, err)
	}
	if all, _ := s.Load(); all["TOKEN"] != canary+"c" {
		t.Errorf("TOKEN = %q", all["TOKEN"])
	}
}

func TestStore_RefusesFileOthersCanRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not exist on Windows")
	}
	s := newStore(t)
	if err := s.Set("TOKEN", canary); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.Load()
	if err == nil {
		t.Fatal("Load accepted a world-readable file")
	}
	if strings.Contains(err.Error(), canary) || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("error = %q", err)
	}
	if s.Set("OTHER", canary) == nil {
		t.Error("Set wrote into a world-readable store")
	}
}

func TestStore_CorruptFileErrorHasNoContent(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path(), []byte(`{"secrets": {"A": `+canary), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.Load()
	if err == nil || strings.Contains(err.Error(), canary) {
		t.Errorf("error = %v", err)
	}
}

func TestStore_RefusesSymbolicLink(t *testing.T) {
	s := newStore(t)
	if err := s.Set("TOKEN", canary); err != nil {
		t.Fatal(err)
	}
	real := s.Path() + ".real"
	if err := os.Rename(s.Path(), real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, s.Path()); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("Load followed a symbolic link")
	}
}
