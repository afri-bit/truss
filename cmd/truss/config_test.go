package main

import (
	"context"
	"github.com/afri-bit/truss/internal/config"
	"github.com/afri-bit/truss/internal/daemon"
	"github.com/afri-bit/truss/internal/runs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Spec 027: truss daemon config shows each setting and where it comes from.
func TestDaemonConfig_ShowsValuesAndSources(t *testing.T) {
	s := newSession(t, false)
	out := s.ok("daemon", "config")
	if !strings.Contains(out, "(not there;") || !strings.Contains(out, "max_runs  4") {
		t.Fatalf("no file:\n%s", out)
	}
	if err := os.WriteFile(filepath.Join(s.home, "config.yaml"), []byte("max_runs: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.env = append(s.env, "TRUSS_DAEMON_HISTORY=50")
	out = s.ok("daemon", "config")
	for _, want := range []string{"max_runs  2               config.yaml", "history   50              TRUSS_DAEMON_HISTORY", "address   127.0.0.1:7878  default"} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %q:\n%s", want, out)
		}
	}
	type row struct{ Key, Value, Source string }
	doc := decode[struct {
		Exists   bool
		Settings []row
	}](t, s.ok("daemon", "config", "--json"))
	if !doc.Exists || len(doc.Settings) != 3 || doc.Settings[1] != (row{"max_runs", "2", "config.yaml"}) {
		t.Fatalf("json = %+v", doc)
	}
	if err := os.WriteFile(filepath.Join(s.home, "config.yaml"), []byte("max_run: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := s.truss("daemon", "config"); code != 1 || !strings.Contains(errOut, `unknown setting "max_run"`) {
		t.Fatalf("bad key: %d %q", code, errOut)
	}
}

// The daemon applies the file when it starts.
func TestDaemonConfig_TheDaemonUsesTheFile(t *testing.T) {
	s := newSession(t, false)
	if err := os.WriteFile(filepath.Join(s.home, "config.yaml"), []byte("address: 127.0.0.1:0\nmax_runs: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		var out, errOut strings.Builder
		done <- RunWithInput(ctx, []string{"daemon", "run"}, s.dir, s.env, nil, &out, &errOut)
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon run did not stop")
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		code, out, _ := s.truss("daemon", "status")
		if code == 0 {
			if !strings.Contains(out, "limit 3 at once") {
				t.Fatalf("status:\n%s", out)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the daemon did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Review of spec 027: start reports a bad setting itself, install refuses a
// flag it cannot carry, and the address rule names its source.
func TestDaemonConfig_ErrorsShowWhereTheyHappen(t *testing.T) {
	s := newSession(t, false)
	if err := os.WriteFile(filepath.Join(s.home, "config.yaml"), []byte("max_run: 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := s.truss("daemon", "start"); code != 1 || !strings.Contains(errOut, `unknown setting "max_run"`) {
		t.Fatalf("start with a bad file: %d %q", code, errOut)
	}
	if err := os.WriteFile(filepath.Join(s.home, "config.yaml"), []byte("address: 10.1.2.3:80\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := s.truss("daemon", "config"); code != 1 || !strings.Contains(errOut, "address (from config.yaml): must be a loopback") {
		t.Fatalf("config with a remote address: %d %q", code, errOut)
	}
	if code, _, errOut := s.truss("daemon", "install", "--history", "5"); code != 1 || !strings.Contains(errOut, "--history goes with daemon start or run") {
		t.Fatalf("install --history: %d %q", code, errOut)
	}
}

// The defaults that daemon config shows are the ones the daemon uses.
func TestDaemonConfig_DefaultsMatchTheDaemon(t *testing.T) {
	cfg, err := config.Resolve(filepath.Join(t.TempDir(), "none.yaml"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Get("address") != daemon.DefaultAddress || cfg.Int("max_runs") != runs.DefaultMaxParallel || cfg.Int("history") != runs.DefaultKeep {
		t.Fatalf("defaults %v differ from the daemon's", cfg.Values)
	}
}
