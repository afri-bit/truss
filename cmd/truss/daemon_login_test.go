package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/loginstart"
)

// fakeUserHome points the user's home directory at a temporary one, so that
// these tests do not read or write the real user's files. It does not stop
// systemctl, launchctl or schtasks from reaching the real login mechanism; the
// tests that could do so use a fake host or stop at the refusal of a temporary binary.
func fakeUserHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestDaemonInstall_RefusesATemporaryBinary(t *testing.T) {
	// The test binary lives in a temporary directory, like a "go run" build.
	// A test binary built to a permanent path would get past the refusal and
	// reach the real login mechanism, so such a run skips this test.
	if exe, err := os.Executable(); err != nil || loginstart.TemporaryReason(exe, os.TempDir()) == "" {
		t.Skip("the test binary is not in a temporary directory")
	}
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

// fakeLogin is a login host that records commands and keeps files in memory.
type fakeLogin struct {
	files map[string]string
	calls []string
}

func (f *fakeLogin) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if strings.Contains(line, "is-active") || strings.Contains(line, "launchctl print") || strings.Contains(line, "/Query") {
		return nil, errors.New("exit status 1")
	}
	return nil, nil
}
func (f *fakeLogin) Read(p string) ([]byte, error) {
	if v, ok := f.files[p]; ok {
		return []byte(v), nil
	}
	return nil, os.ErrNotExist
}
func (f *fakeLogin) Write(p string, d []byte, _ os.FileMode) error {
	f.files[p] = string(d)
	return nil
}
func (f *fakeLogin) Remove(p string) error { delete(f.files, p); return nil }

func (f *fakeLogin) host(goos string) loginstart.Host {
	return loginstart.Host{OS: goos, UserHome: "/home/me", UID: 501, User: `PC\me`, Scratch: "/scratch", Run: f, Files: f}
}

func installOutput(t *testing.T, goos string, running bool) (int, string, string, *fakeLogin) {
	t.Helper()
	s := newSession(t, false)
	f := &fakeLogin{files: map[string]string{}}
	var stdout, stderr strings.Builder
	in := invocation{cwd: s.dir, env: s.env, stdout: &stdout, stderr: &stderr}
	h, err := home.Resolve(s.env)
	if err != nil {
		t.Fatal(err)
	}
	spec := loginstart.Spec{Binary: "/usr/local/bin/truss", MaxRuns: 2}
	code := in.installLogin(context.Background(), f.host(goos), h, spec, running)
	return code, stdout.String(), stderr.String(), f
}

func TestInstallLogin_StartsTheDaemonWhenNoneRuns(t *testing.T) {
	code, out, errOut, f := installOutput(t, "linux", false)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(strings.Join(f.calls, "\n"), "restart truss.service") {
		t.Errorf("calls = %v", f.calls)
	}
	if strings.Contains(out, "left alone") || !strings.Contains(out, "running under systemd") {
		t.Errorf("stdout = %q", out)
	}
}

func TestInstallLogin_LeavesAHandStartedDaemonAlone(t *testing.T) {
	code, out, errOut, f := installOutput(t, "linux", true)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "restart") {
			t.Errorf("must not start a second daemon: %s", c)
		}
	}
	if !strings.Contains(out, "left alone") || !strings.Contains(out, "next login") {
		t.Errorf("stdout = %q", out)
	}
}

func TestInstallLogin_ReportsAFailure(t *testing.T) {
	s := newSession(t, false)
	f := &fakeLogin{files: map[string]string{}}
	var stdout, stderr strings.Builder
	in := invocation{cwd: s.dir, env: s.env, stdout: &stdout, stderr: &stderr}
	h, _ := home.Resolve(s.env)
	host := f.host("freebsd")
	if code := in.installLogin(context.Background(), host, h, loginstart.Spec{Binary: "/x/truss"}, false); code != 1 || !strings.Contains(stderr.String(), "supported on") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}
