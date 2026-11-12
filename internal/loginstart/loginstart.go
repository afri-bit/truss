// Package loginstart installs TRUSS's background daemon as a login item, so
// schedules keep working after a reboot without anyone typing "truss daemon start".
//
// The files each platform needs (a systemd user unit, a launchd agent, a Task
// Scheduler definition) are produced by pure functions of a [Spec]. Everything
// that touches the operating system goes through the [Runner] and [Files]
// interfaces of a [Host], so the logic is testable on every platform and the
// real implementations stay thin.
//
// Nothing here needs root or administrator rights: every item is per user.
package loginstart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Marker is written into every file and task this package creates. Uninstall
// and status only trust, and only remove, things that carry it.
const Marker = "Managed by truss daemon install"

// Names of the things install creates.
const (
	// UnitName is the systemd user unit, in ~/.config/systemd/user.
	UnitName = "truss.service"
	// Label is the launchd label; the plist is <Label>.plist.
	Label = "io.github.afri-bit.truss"
	// TaskName is the Task Scheduler task.
	TaskName = "truss-daemon"
)

// ErrUnsupported is returned on an operating system without a login mechanism here.
var ErrUnsupported = errors.New("start at login is supported on Linux (systemd), macOS (launchd) and Windows (Task Scheduler) only")

// Spec is everything the generated files depend on.
type Spec struct {
	// Binary is the absolute path of the truss executable.
	Binary string
	// Address is passed as --address; empty keeps the daemon's default.
	Address string
	// MaxRuns is passed as --max-runs; 0 keeps the daemon's default.
	MaxRuns int
	// Home is the value of TRUSS_HOME to give the daemon; empty keeps the default home.
	Home string
	// LogPath is where launchd writes the daemon's output (macOS only).
	LogPath string
	// User is the account the Windows task belongs to, as DOMAIN\name.
	User string
}

// Args returns the arguments after the binary: "daemon run" plus the flags set.
func (s Spec) Args() []string {
	args := []string{"daemon", "run"}
	if s.Address != "" {
		args = append(args, "--address", s.Address)
	}
	if s.MaxRuns > 0 {
		args = append(args, "--max-runs", strconv.Itoa(s.MaxRuns))
	}
	return args
}

func (s Spec) validate() error {
	if s.Binary == "" {
		return errors.New("no path to the truss binary")
	}
	for name, v := range map[string]string{"binary path": s.Binary, "address": s.Address, "TRUSS_HOME": s.Home, "log path": s.LogPath, "user name": s.User} {
		if strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return fmt.Errorf("the %s contains a control character", name)
		}
	}
	return nil
}

// Runner runs an operating system command and returns its combined output.
// A command that exits non-zero returns a non-nil error.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Files is the file system as install uses it.
type Files interface {
	// Read returns the contents, or an error satisfying errors.Is(err, os.ErrNotExist).
	Read(path string) ([]byte, error)
	// Write creates parent directories and replaces the file.
	Write(path string, data []byte, perm os.FileMode) error
	// Remove deletes the file; a missing file is not an error.
	Remove(path string) error
}

// Host is the machine install works on.
type Host struct {
	OS       string // runtime.GOOS
	UserHome string // the user's home directory
	UID      int    // numeric user id (macOS launchd domain)
	Scratch  string // a private directory for short-lived files (Windows task XML)
	// Wait is the pause between attempts to reload a launchd agent; zero means none.
	Wait  time.Duration
	Run   Runner
	Files Files
}

// Result describes what Install or Uninstall did, one line per message.
type Result struct {
	Messages []string
}

func (r *Result) add(format string, a ...any) {
	r.Messages = append(r.Messages, fmt.Sprintf(format, a...))
}

// Supported reports whether the host's operating system has a login mechanism here.
func (h Host) Supported() bool {
	return h.OS == "linux" || h.OS == "darwin" || h.OS == "windows"
}

// Installed reports whether login start created by Install is present. It
// reads files where it can and asks the OS only on Windows.
func Installed(ctx context.Context, h Host) (bool, error) {
	switch h.OS {
	case "linux":
		return h.hasMarker(h.unitPath())
	case "darwin":
		return h.hasMarker(h.plistPath())
	case "windows":
		out, err := h.Run.Run(ctx, "schtasks.exe", SchtasksQueryArgs()...)
		switch {
		case err != nil: // no such task (or no schtasks: install will say so)
			return false, nil
		case !strings.Contains(string(out), Marker):
			return false, fmt.Errorf("a scheduled task named %q exists but was not created by truss daemon install; leaving it alone", TaskName)
		}
		return true, nil
	}
	return false, ErrUnsupported
}

// hasMarker reports whether path exists and was written by Install. A file
// that exists without the marker is somebody else's and is an error.
func (h Host) hasMarker(path string) (bool, error) {
	data, err := h.Files.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(data), Marker) {
		return false, fmt.Errorf("%s exists but was not created by truss daemon install; leaving it alone", path)
	}
	return true, nil
}

// Install makes the daemon start at login. With startNow it also starts it
// through the login mechanism now; callers pass false when a daemon already runs,
// because a second daemon would exit and be restarted in a loop.
func Install(ctx context.Context, h Host, s Spec, startNow bool) (Result, error) {
	if err := s.validate(); err != nil {
		return Result{}, err
	}
	switch h.OS {
	case "linux":
		return h.installSystemd(ctx, s, startNow)
	case "darwin":
		return h.installLaunchd(ctx, s, startNow)
	case "windows":
		return h.installTask(ctx, s, startNow)
	}
	return Result{}, ErrUnsupported
}

// Uninstall removes what Install created, and only that. It stops the daemon
// the login mechanism started; a daemon started by "truss daemon start" is untouched.
func Uninstall(ctx context.Context, h Host) (Result, error) {
	switch h.OS {
	case "linux":
		return h.uninstallSystemd(ctx)
	case "darwin":
		return h.uninstallLaunchd(ctx)
	case "windows":
		return h.uninstallTask(ctx)
	}
	return Result{}, ErrUnsupported
}

// run runs a command and wraps a failure with the command line and its output.
func (h Host) run(ctx context.Context, name string, args ...string) error {
	out, err := h.Run.Run(ctx, name, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			msg = ": " + msg
		}
		return fmt.Errorf("%s %s failed (%v)%s", name, strings.Join(args, " "), err, msg)
	}
	return nil
}
