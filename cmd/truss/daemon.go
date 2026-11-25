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
	"github.com/afri-bit/truss/internal/config"
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
	switch {
	case sub == "logs":
		return in.daemonLogs(ctx, in.args[1:])
	case sub == "status" && len(in.args) == 2 && in.args[1] == "--json":
		return in.daemonStatusJSON(ctx)
	}
	if sub == "config" {
		return in.daemonConfig(in.args[1:])
	}
	pos, flags, err := options(in.args[1:], []string{"address", "max-runs", "history"}, []string{"wait"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	_, wait := flags["wait"]
	delete(flags, "wait")
	if (sub == "stop" || sub == "status" || sub == "uninstall") && len(flags) != 0 || wait && sub != "stop" {
		return in.usage()
	}
	if _, ok := flags["history"]; ok && sub != "start" && sub != "run" {
		// A login item does not carry --history: it belongs in config.yaml.
		return in.fail(errors.New("--history goes with daemon start or run; for the daemon that starts at login, set history in config.yaml (see truss daemon config)"))
	}
	if text, ok := flags["max-runs"]; ok {
		if n, err := strconv.Atoi(text); err != nil || n < 1 {
			return in.fail(fmt.Errorf("--max-runs must be a whole number of at least 1, got %q", text))
		}
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	maxRuns, _ := strconv.Atoi(flags["max-runs"])
	switch sub {
	case "run":
		// The settings resolve here, in the process that serves: flag, then
		// environment, then config.yaml, then default (spec 027).
		cfg, err := resolveConfig(h, flags, in.env)
		if err != nil {
			return in.fail(err)
		}
		return in.daemonRun(ctx, h, cfg)
	case "start":
		// Check here too, so a bad file or flag is reported on the screen,
		// not only in the log of a daemon that stopped at once.
		if _, err := resolveConfig(h, flags, in.env); err != nil {
			return in.fail(err)
		}
		return in.daemonStart(ctx, h, in.args[1:])
	case "stop":
		return in.daemonStop(ctx, h, wait)
	case "status":
		return in.daemonStatus(ctx, h)
	case "install":
		return in.daemonInstall(ctx, h, flags["address"], maxRuns)
	case "uninstall":
		return in.daemonUninstall(ctx, h)
	}
	return in.usage()
}

// daemonRun serves in the foreground until interrupted.
func (in invocation) daemonRun(ctx context.Context, h home.Dir, cfg config.Config) int {
	d, err := daemon.Start(ctx, daemon.Options{
		Home: h, Address: cfg.Get("address"), MaxParallelRuns: cfg.Int("max_runs"), KeepRuns: cfg.Int("history"),
		Version: Version, Env: in.env, Log: in.stderr,
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
	rotateLog(h.DaemonLog())
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

func (in invocation) daemonStop(ctx context.Context, h home.Dir, wait bool) int {
	c, err := api.Dial(ctx, h)
	if errors.Is(err, api.ErrNotRunning) {
		fmt.Fprintln(in.stdout, "daemon is not running")
		return 0
	}
	if err != nil {
		return in.fail(err)
	}
	// --wait lets running and queued runs finish first (spec 028). A run a
	// schedule starts meanwhile is waited for too.
	for said := -1; wait; {
		st, err := c.Status(ctx)
		if err != nil {
			return in.fail(err)
		}
		active := st.RunningRuns + st.QueuedRuns
		if active == 0 {
			break
		}
		if active != said {
			fmt.Fprintf(in.stdout, "waiting for %d run(s) to finish (Ctrl-C to stop waiting)\n", active)
			said = active
		}
		select {
		case <-ctx.Done():
			return in.fail(errors.New("stopped waiting; the daemon is still running"))
		case <-time.After(500 * time.Millisecond):
		}
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
		in.loginLine(ctx, h)
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
	fmt.Fprintf(in.stdout, "  flows      %d\n", st.Pipelines)
	fmt.Fprintf(in.stdout, "  runs       %d running, %d queued (limit %d at once)\n", st.RunningRuns, st.QueuedRuns, st.MaxParallelRuns)
	in.loginLine(ctx, h)
	return 0
}

// desktop prints the address of TRUSS Desktop and opens it in the browser.
// With --demo it needs no daemon and serves the page with sample data instead.
func (in invocation) desktop(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"no-open", "demo"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	if flags["demo"] != "" {
		return in.desktopDemo(ctx, flags["no-open"] == "")
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

// daemonConfig prints each daemon setting, its value and where it comes from
// (spec 027). It reads the file and the environment as truss daemon start
// would; flags given to start are not known here.
func (in invocation) daemonConfig(args []string) int {
	pos, flags, err := options(args, nil, []string{"json"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	cfg, err := resolveConfig(h, nil, in.env)
	if err != nil {
		return in.fail(err)
	}
	if flags["json"] != "" {
		type row struct {
			Key     string `json:"key"`
			Value   string `json:"value"`
			Source  string `json:"source"`
			Default string `json:"default"`
			Env     string `json:"env"`
			Flag    string `json:"flag"`
		}
		out := struct {
			File   string `json:"file"`
			Exists bool   `json:"exists"`
			Values []row  `json:"settings"`
		}{File: cfg.Path, Exists: cfg.Exists, Values: []row{}}
		for _, v := range cfg.Values {
			out.Values = append(out.Values, row{v.Key, v.Value, v.Source, v.Default, v.Env, "--" + v.Flag})
		}
		return writeJSON(in.stdout, out)
	}
	state := " (not there; every value is a default or from the environment)"
	if cfg.Exists {
		state = ""
	}
	fmt.Fprintf(in.stdout, "file  %s%s\n\n", clean(cfg.Path), state)
	rows := [][]string{{"SETTING", "VALUE", "SOURCE"}}
	for _, v := range cfg.Values {
		rows = append(rows, []string{v.Key, v.Value, v.Source})
	}
	for _, l := range columns(rows, screenWidth(in.env)) {
		fmt.Fprintln(in.stdout, clean(l))
	}
	fmt.Fprintln(in.stdout, "\nSet a value in the file, with a variable or with a flag of truss daemon start:")
	for _, v := range cfg.Values {
		fmt.Fprintf(in.stdout, "  %-9s %s or --%s   %s\n", v.Key, v.Env, v.Flag, v.Help)
	}
	fmt.Fprintln(in.stdout, "A flag wins over a variable, which wins over the file.")
	fmt.Fprintln(in.stdout, "Changes take effect when the daemon starts again.")
	return 0
}

// resolveConfig resolves the daemon's settings and checks the address rule
// of spec 006 (loopback only), naming where a bad value came from.
func resolveConfig(h home.Dir, flags map[string]string, env []string) (config.Config, error) {
	cfg, err := config.Resolve(h.Config(), flags, env)
	if err != nil {
		return cfg, err
	}
	for _, v := range cfg.Values {
		if v.Key == "address" && daemon.CheckAddress(v.Value) != nil {
			return cfg, fmt.Errorf("address (from %s): must be a loopback host:port, such as 127.0.0.1:7878; the daemon only serves this machine", v.Describe())
		}
	}
	return cfg, nil
}

// maxDaemonLog is the size at which the daemon's log is set aside when the
// daemon starts: it becomes daemon.log.1, replacing an older one (spec 028).
const maxDaemonLog = 10 << 20

// rotateLog keeps the daemon log bounded across starts.
func rotateLog(path string) {
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() > maxDaemonLog {
		_ = os.Rename(path, path+".1")
	}
}
