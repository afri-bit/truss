package main

import (
	"context"
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
