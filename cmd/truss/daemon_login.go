package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/daemon"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/loginstart"
)

// loginHost describes this machine for the login-item installer.
func loginHost(h home.Dir) (loginstart.Host, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return loginstart.Host{}, fmt.Errorf("find your home directory: %w", err)
	}
	host := loginstart.Host{
		OS: runtime.GOOS, UserHome: userHome, UID: os.Getuid(), Scratch: string(h),
		Elevated: os.Geteuid() == 0, // -1 on Windows
		Wait:     300 * time.Millisecond, Run: loginstart.ExecRunner{}, Files: loginstart.OSFiles{},
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		host.ConfigHome = x
	}
	if u, err := user.Current(); err == nil {
		host.User = u.Username
	}
	return host, nil
}

// daemonInstall makes the daemon start at login.
func (in invocation) daemonInstall(ctx context.Context, h home.Dir, address string, maxRuns int) int {
	if address != "" {
		if err := daemon.CheckAddress(address); err != nil {
			return in.fail(err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return in.fail(fmt.Errorf("find the truss binary: %w", err))
	}
	// Check the path as given and where it leads. Record the path as given: a
	// package manager's symbolic link (Homebrew's bin/truss) survives upgrades,
	// the versioned directory behind it does not.
	for _, p := range []string{exe, resolved(exe)} {
		if reason := loginstart.TemporaryReason(p, os.TempDir()); reason != "" {
			return in.fail(fmt.Errorf("%s looks temporary: %s. A login item that points there stops working; install truss in a permanent place and run this command from there", p, reason))
		}
	}
	host, err := loginHost(h)
	if err != nil {
		return in.fail(err)
	}
	if !host.Supported() {
		return in.fail(loginstart.ErrUnsupported)
	}
	spec := loginstart.Spec{Binary: exe, Address: address, MaxRuns: maxRuns, LogPath: h.DaemonLog()}
	if explicitHome(in.env) {
		spec.Home = string(h)
	}
	running := true
	if _, err := api.Dial(ctx, h); errors.Is(err, api.ErrNotRunning) {
		running = false
	} else if err != nil {
		// Something answers but not as we expect; starting a second daemon would be wrong.
		return in.fail(fmt.Errorf("cannot tell whether a daemon is running: %w", err))
	}
	return in.installLogin(ctx, host, h, spec, running)
}

func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}

// installLogin installs the login item on host. A daemon that is already
// running (started by hand) is left alone: starting a second one through the
// login item would only make it exit and restart.
func (in invocation) installLogin(ctx context.Context, host loginstart.Host, h home.Dir, spec loginstart.Spec, running bool) int {
	if err := h.Ensure(); err != nil {
		return in.fail(err)
	}
	res, err := loginstart.Install(ctx, host, spec, !running)
	for _, m := range res.Messages {
		fmt.Fprintln(in.stdout, m)
	}
	if err != nil {
		return in.fail(err)
	}
	if running && !res.Restarted {
		fmt.Fprintln(in.stdout, "the daemon that is running now was left alone")
	}
	return 0
}

// daemonUninstall removes what daemon install created.
func (in invocation) daemonUninstall(ctx context.Context, h home.Dir) int {
	host, err := loginHost(h)
	if err != nil {
		return in.fail(err)
	}
	res, err := loginstart.Uninstall(ctx, host)
	for _, m := range res.Messages {
		fmt.Fprintln(in.stdout, m)
	}
	if err != nil {
		return in.fail(err)
	}
	return 0
}

// loginLine prints whether the daemon starts at login. It prints nothing where
// that cannot be known, so status never fails because of it.
func (in invocation) loginLine(ctx context.Context, h home.Dir) {
	host, err := loginHost(h)
	if err != nil || !host.Supported() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch ok, err := loginstart.Installed(ctx, host); {
	case err != nil:
		fmt.Fprintf(in.stdout, "  at login   unknown (%s)\n", err.Error())
	case ok:
		fmt.Fprintln(in.stdout, "  at login   installed (remove with: truss daemon uninstall)")
	default:
		fmt.Fprintln(in.stdout, "  at login   not installed (install with: truss daemon install)")
	}
}

// explicitHome reports whether the environment names a TRUSS home.
func explicitHome(env []string) bool {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, home.EnvVar+"="); ok && v != "" {
			return true
		}
	}
	return false
}
