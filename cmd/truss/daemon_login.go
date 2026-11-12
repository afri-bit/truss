package main

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

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
	return loginstart.Host{
		OS: runtime.GOOS, UserHome: userHome, UID: os.Getuid(), Scratch: string(h),
		Run: loginstart.ExecRunner{}, Files: loginstart.OSFiles{},
	}, nil
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
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if reason := loginstart.TemporaryReason(exe, os.TempDir()); reason != "" {
		return in.fail(fmt.Errorf("%s looks temporary: %s. A login item that points there stops working; install truss in a permanent place and run this command from there", exe, reason))
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
	if u, err := user.Current(); err == nil {
		spec.User = u.Username
	}
	if err := h.Ensure(); err != nil {
		return in.fail(err)
	}
	// A daemon that is already up (started by hand) keeps running; starting a
	// second one through the login item would only make it exit and restart.
	_, dialErr := api.Dial(ctx, h)
	res, err := loginstart.Install(ctx, host, spec, dialErr != nil)
	for _, m := range res.Messages {
		fmt.Fprintln(in.stdout, m)
	}
	if err != nil {
		return in.fail(err)
	}
	if dialErr == nil {
		fmt.Fprintln(in.stdout, "a daemon started by hand is still running and was left alone")
	}
	return 0
}

// daemonUninstall removes what daemon install created.
func (in invocation) daemonUninstall(ctx context.Context) int {
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
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
func (in invocation) loginLine(ctx context.Context) {
	h, err := home.Resolve(in.env)
	if err != nil {
		return
	}
	host, err := loginHost(h)
	if err != nil || !host.Supported() {
		return
	}
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
