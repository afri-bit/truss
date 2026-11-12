package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/loginstart"
)

// fakeUserHome points the user's home directory at a temporary one, so that
// nothing in these tests can see or touch the real login items.
func fakeUserHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestDaemonInstall_RefusesATemporaryBinary(t *testing.T) {
	// The test binary lives in a temporary directory, like a "go run" build.
	userHome := fakeUserHome(t)
	s := newSession(t, false)
	code, out, errOut := s.truss("daemon", "install")
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	if !strings.Contains(errOut, "looks temporary") || !strings.Contains(errOut, "permanent place") {
		t.Errorf("stderr = %q, want the reason and what to do", errOut)
	}
	if entries, _ := os.ReadDir(userHome); len(entries) != 0 {
		t.Errorf("nothing may be written under the user's home: %v", entries)
	}
}

func TestDaemonInstall_RejectsBadFlags(t *testing.T) {
	fakeUserHome(t)
	s := newSession(t, false)
	for _, args := range [][]string{
		{"daemon", "install", "--address", "0.0.0.0:7878"},
		{"daemon", "install", "--address", "nonsense"},
		{"daemon", "install", "--max-runs", "0"},
		{"daemon", "install", "extra"},
		{"daemon", "uninstall", "--address", "127.0.0.1:1"},
	} {
		if code, _, errOut := s.truss(args...); code != 1 || errOut == "" {
			t.Errorf("%v: exit %d, stderr %q; want exit 1 and a message", args, code, errOut)
		}
	}
}

func TestDaemonUninstall_NothingInstalled(t *testing.T) {
	fakeUserHome(t)
	s := newSession(t, false)
	if runtime.GOOS == "windows" {
		t.Skip("uninstall asks Task Scheduler on Windows; covered by the fake runner tests")
	}
	if out := s.ok("daemon", "uninstall"); !strings.Contains(out, "not installed") {
		t.Errorf("stdout = %q", out)
	}
}

func TestDaemonStatus_SaysWhetherLoginStartIsInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("status asks Task Scheduler on Windows")
	}
	userHome := fakeUserHome(t)
	s := newSession(t, true)
	if out := s.ok("daemon", "status"); !strings.Contains(out, "at login   not installed") {
		t.Errorf("running daemon, nothing installed:\n%s", out)
	}

	path := filepath.Join(userHome, ".config", "systemd", "user", loginstart.UnitName)
	if runtime.GOOS == "darwin" {
		path = filepath.Join(userHome, "Library", "LaunchAgents", loginstart.Label+".plist")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# "+loginstart.Marker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := s.ok("daemon", "status"); !strings.Contains(out, "at login   installed") {
		t.Errorf("running daemon, installed:\n%s", out)
	}
}

func TestDaemonStatus_NotRunningStillExitsOneAndShowsLoginStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("status asks Task Scheduler on Windows")
	}
	fakeUserHome(t)
	s := newSession(t, false)
	code, out, _ := s.truss("daemon", "status")
	if code != 1 || !strings.Contains(out, "daemon is not running") || !strings.Contains(out, "at login   not installed") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
}
