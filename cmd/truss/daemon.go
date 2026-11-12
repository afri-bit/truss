package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/daemon"
	"github.com/afri-bit/truss/internal/home"
)

// startWait bounds how long "daemon start" and "daemon stop" wait.
const startWait = 15 * time.Second

func (in invocation) daemon(ctx context.Context) int {
	if len(in.args) == 0 {
		return in.usage()
	}
	sub := in.args[0]
	pos, flags, err := options(in.args[1:], []string{"address", "max-runs"}, nil)
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	if (sub == "stop" || sub == "status" || sub == "uninstall") && len(flags) != 0 {
		return in.usage()
	}
	maxRuns := 0
	if text, ok := flags["max-runs"]; ok {
		if maxRuns, err = strconv.Atoi(text); err != nil || maxRuns < 1 {
			return in.fail(fmt.Errorf("--max-runs must be a whole number of at least 1, got %q", text))
		}
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	switch sub {
	case "run":
		return in.daemonRun(ctx, h, flags["address"], maxRuns)
	case "start":
		return in.daemonStart(ctx, h, in.args[1:])
	case "stop":
		return in.daemonStop(ctx, h)
	case "status":
		return in.daemonStatus(ctx, h)
	case "install":
		return in.daemonInstall(ctx, h, flags["address"], maxRuns)
	case "uninstall":
		return in.daemonUninstall(ctx)
	}
	return in.usage()
}

// daemonRun serves in the foreground until interrupted.
func (in invocation) daemonRun(ctx context.Context, h home.Dir, address string, maxRuns int) int {
	d, err := daemon.Start(ctx, daemon.Options{
		Home: h, Address: address, MaxParallelRuns: maxRuns, Version: Version, Env: in.env, Log: in.stderr,
	})
	if err != nil {
		return in.fail(err)
	}
	if err := d.Serve(ctx); err != nil {
		return in.fail(err)
	}
	return 0
}

// daemonStart launches "truss daemon run" as a background process that
// outlives this command, and waits until it answers.
func (in invocation) daemonStart(ctx context.Context, h home.Dir, flags []string) int {
	if c, err := api.Dial(ctx, h); err == nil {
		fmt.Fprintf(in.stdout, "daemon already running (pid %d, %s)\n", c.Endpoint().PID, c.Endpoint().Address)
		return 0
	}
	if err := h.Ensure(); err != nil {
		return in.fail(err)
	}
	exe, err := os.Executable()
	if err != nil {
		return in.fail(fmt.Errorf("find the truss binary: %w", err))
	}
	logFile, err := os.OpenFile(h.DaemonLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return in.fail(fmt.Errorf("open daemon log: %w", err))
	}
	defer func() { _ = logFile.Close() }()

	// #nosec G204 -- this starts the truss binary itself, with flags that were parsed above.
	cmd := exec.Command(exe, append([]string{"daemon", "run"}, flags...)...)
	cmd.Env = in.env
	cmd.Dir = string(h)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return in.fail(fmt.Errorf("start daemon: %w", err))
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()

	deadline := time.After(startWait)
	for {
		if c, err := api.Dial(ctx, h); err == nil && c.Endpoint().PID == cmd.Process.Pid {
			fmt.Fprintf(in.stdout, "daemon started (pid %d, %s)\n", c.Endpoint().PID, c.Endpoint().Address)
			return 0
		}
		select {
		case <-exited:
			return in.fail(fmt.Errorf("the daemon stopped right after starting; see %s", h.DaemonLog()))
		case <-deadline:
			return in.fail(fmt.Errorf("the daemon did not answer within %s; see %s", startWait, h.DaemonLog()))
		case <-ctx.Done():
			return in.fail(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (in invocation) daemonStop(ctx context.Context, h home.Dir) int {
	c, err := api.Dial(ctx, h)
	if errors.Is(err, api.ErrNotRunning) {
		fmt.Fprintln(in.stdout, "daemon is not running")
		return 0
	}
	if err != nil {
		return in.fail(err)
	}
	if err := c.Shutdown(ctx); err != nil {
		return in.fail(err)
	}
	deadline := time.After(startWait)
	for {
		if _, err := os.Stat(h.Endpoint()); os.IsNotExist(err) {
			fmt.Fprintln(in.stdout, "daemon stopped")
			return 0
		}
		select {
		case <-deadline:
			return in.fail(fmt.Errorf("the daemon is still stopping its runs; check again with: truss daemon status"))
		case <-ctx.Done():
			return in.fail(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (in invocation) daemonStatus(ctx context.Context, h home.Dir) int {
	c, err := api.Dial(ctx, h)
	if errors.Is(err, api.ErrNotRunning) {
		fmt.Fprintln(in.stdout, "daemon is not running (start it with: truss daemon start)")
		in.loginLine(ctx)
		return 1
	}
	if err != nil {
		return in.fail(err)
	}
	st, err := c.Status(ctx)
	if err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "daemon is running\n")
	fmt.Fprintf(in.stdout, "  version    %s\n", st.Version)
	fmt.Fprintf(in.stdout, "  pid        %d\n", st.PID)
	fmt.Fprintf(in.stdout, "  address    %s\n", st.Address)
	fmt.Fprintf(in.stdout, "  home       %s\n", st.Home)
	fmt.Fprintf(in.stdout, "  started    %s\n", local(&st.StartedAt))
	fmt.Fprintf(in.stdout, "  pipelines  %d\n", st.Pipelines)
	fmt.Fprintf(in.stdout, "  runs       %d running, %d queued (limit %d at once)\n", st.RunningRuns, st.QueuedRuns, st.MaxParallelRuns)
	in.loginLine(ctx)
	return 0
}

// desktop prints the address of TRUSS Desktop and opens it in the browser.
func (in invocation) desktop(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"no-open"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	// The token travels in the fragment, which browsers never send to a server.
	url := "http://" + c.Endpoint().Address + "/#token=" + c.Endpoint().Token
	fmt.Fprintf(in.stdout, "TRUSS Desktop: %s\n", url)
	if flags["no-open"] != "" {
		return 0
	}
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(in.stderr, "could not open a browser (%s); open the address above yourself\n", err.Error())
	}
	return 0
}
