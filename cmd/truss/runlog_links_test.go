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

// assertNoTemporaryLog fails if a temporary run log (run.log*.tmp, any name
// this or an earlier version used) is left in the .truss folder trussDir.
func assertNoTemporaryLog(t *testing.T, trussDir string) {
	t.Helper()
	left, err := filepath.Glob(filepath.Join(trussDir, "run.log*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Fatalf("temporary run log left behind: %s", strings.Join(left, ", "))
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
	assertNoTemporaryLog(t, filepath.Join(dir, ".truss"))
}

func TestRun_FailedRunLeavesNoTemporaryLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(validPipeline("exit 3")), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := invoke(t, dir, []string{"run", path}); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	readRunLog(t, dir) // FR-011: a failed run still saves its log
	assertNoTemporaryLog(t, filepath.Join(dir, ".truss"))
}

// A link at .truss (or .truss/data) would send the run log, data and cache into
// a folder the checkout chose, such as ~/.bashrc.d. Such a folder is refused
// before any Step starts.
func TestRun_LinkedTrussFoldersAreRefused(t *testing.T) {
	for _, linked := range []string{".truss", filepath.Join(".truss", "data"), filepath.Join(".truss", "data", "cache")} {
		t.Run(filepath.ToSlash(linked), func(t *testing.T) {
			dir := t.TempDir()
			victim := t.TempDir()
			const keep = "do not touch\n"
			if err := os.WriteFile(filepath.Join(victim, "run.log"), []byte(keep), 0o600); err != nil {
				t.Fatal(err)
			}
			if parent := filepath.Dir(filepath.Join(dir, linked)); parent != dir {
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			plantLink(t, victim, filepath.Join(dir, linked))
			marker := filepath.Join(dir, "ran.txt")
			run := "echo x > '" + marker + "'"
			if runtime.GOOS == "windows" {
				run = "echo x> \"" + marker + "\""
			}
			path := filepath.Join(dir, "pipeline.yaml")
			if err := os.WriteFile(path, []byte(validPipeline(run)), 0o644); err != nil {
				t.Fatal(err)
			}

			code, _, errOut := invoke(t, dir, []string{"run", path})
			if code != 1 || !strings.Contains(errOut, "link") {
				t.Fatalf("exit = %d, stderr = %q; want 1 and an error naming the link", code, errOut)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("a step ran: %v", err)
			}
			entries, err := os.ReadDir(victim)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("files appeared in the link target: %v", entries)
			}
			if got, _ := os.ReadFile(filepath.Join(victim, "run.log")); string(got) != keep {
				t.Fatalf("run.log in the link target changed to %q", got)
			}
		})
	}
}

// Two runs that start together in a fresh checkout both create .truss and its
// data folders; finding a folder the other run just made is not an error.
func TestRun_TwoRunsInAFreshFolderBothSucceed(t *testing.T) {
	for i := range 20 {
		dir := t.TempDir()
		path := filepath.Join(dir, "pipeline.yaml")
		if err := os.WriteFile(path, []byte(validPipeline(echoHello())), 0o644); err != nil {
			t.Fatal(err)
		}
		type result struct {
			code   int
			stderr string
		}
		results := make(chan result, 2)
		for range 2 {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
				defer cancel()
				var stdout, stderr bytes.Buffer
				code := Run(ctx, []string{"run", path}, dir, os.Environ(), &stdout, &stderr)
				results <- result{code, stderr.String()}
			}()
		}
		for range 2 {
			if r := <-results; r.code != 0 {
				t.Fatalf("attempt %d: exit = %d, stderr = %s", i, r.code, r.stderr)
			}
		}
	}
}
