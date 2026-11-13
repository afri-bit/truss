package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/afri-bit/truss/internal/pipeline"
)

// usageText is the CLI usage printed for help and for unknown invocations.
const usageText = `TRUSS runs pipelines on your machine.

usage: truss <command> [arguments]

Run here, without the daemon:
  run [<path>] [--track]              run a pipeline definition (default: truss.yaml);
                                      --track also reports it to the daemon (needs it)
  validate [<path>]                   check a definition without running it

The daemon:
  daemon start [--address <host:port>] [--max-runs <n>]
  daemon stop | status                manage the background service
  daemon run                          run the service in this terminal
  daemon install [--address <host:port>] [--max-runs <n>]
  daemon uninstall                    start the daemon at login, or stop doing so
  desktop [--no-open]                 open TRUSS Desktop in the browser

Secrets (no daemon needed; values come from standard input, never an argument):
  secret set <name>                   store a secret
  secret list                         list secret names
  secret remove <name>                delete a secret

Registered pipelines (these need the daemon):
  register [<path>] [--name <name>]   make a definition known to the daemon
  unregister <name>                   forget it; its run history stays
  pipelines                           list registered pipelines
  pause <name> | resume <name>        stop or restart a pipeline's schedules
  trigger <name> [--follow]           start a run now
  runs [<name>]                       list runs, newest first
  logs <run-id> [--follow]            show a run log
  cancel <run-id>                     end a queued or running run

Files only (no daemon needed):
  cache clear <name>                  remove a pipeline's cache folder; the next run recreates it

  help | version
`

// invocation is everything one CLI call may use. Commands write only to
// stdout and stderr and return the process exit code.
type invocation struct {
	args   []string
	cwd    string
	env    []string
	stdin  io.Reader // nil reads as empty
	stdout io.Writer
	stderr io.Writer
}

func (in invocation) fail(err error) int {
	fmt.Fprintf(in.stderr, "error: %s\n", err.Error())
	return 1
}

func (in invocation) usage() int {
	fmt.Fprint(in.stderr, usageText)
	return 1
}

// Run dispatches one CLI invocation and returns the process exit code.
// Help and version write to stdout and do not start a run.
func Run(ctx context.Context, args []string, cwd string, env []string, stdout, stderr io.Writer) int {
	return RunWithInput(ctx, args, cwd, env, nil, stdout, stderr)
}

// RunWithInput is Run with a standard input, which `secret set` reads.
func RunWithInput(ctx context.Context, args []string, cwd string, env []string, stdin io.Reader, stdout, stderr io.Writer) int {
	in := invocation{cwd: cwd, env: env, stdin: stdin, stdout: stdout, stderr: stderr}
	switch {
	case isHelp(args):
		fmt.Fprint(stdout, usageText)
		return 0
	case isVersion(args):
		fmt.Fprintln(stdout, Version)
		return 0
	case len(args) == 0:
		return in.usage()
	}
	in.args = args[1:]
	switch args[0] {
	case "run":
		// --track is the only option of run; any other argument is the path,
		// so a file named like a flag still runs (spec 001).
		var pos []string
		track := false
		for _, a := range in.args {
			if a == "--track" {
				track = true
			} else {
				pos = append(pos, a)
			}
		}
		if len(pos) > 1 {
			return in.usage()
		}
		return runLocal(ctx, pos, track, cwd, env, stdout, stderr)
	case "secret":
		return in.secret(ctx)
	case "validate":
		return in.validate(ctx)
	case "daemon":
		return in.daemon(ctx)
	case "desktop":
		return in.desktop(ctx)
	case "register":
		return in.register(ctx)
	case "unregister":
		return in.unregister(ctx)
	case "pipelines":
		return in.pipelines(ctx)
	case "pause":
		return in.setPaused(ctx, true)
	case "resume":
		return in.setPaused(ctx, false)
	case "trigger":
		return in.trigger(ctx)
	case "runs":
		return in.runs(ctx)
	case "logs":
		return in.logs(ctx)
	case "cancel":
		return in.cancel(ctx)
	case "cache":
		return in.cache()
	}
	return in.usage()
}

func isHelp(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help"))
}

func isVersion(args []string) bool {
	return len(args) == 1 && (args[0] == "version" || args[0] == "--version")
}

func isRun(args []string) bool {
	return len(args) >= 1 && args[0] == "run"
}

// definitionPath returns the definition a command works on: the first
// argument, or truss.yaml in the working directory.
func definitionPath(cwd string, args []string) string {
	if len(args) == 0 {
		return filepath.Join(cwd, "truss.yaml")
	}
	if filepath.IsAbs(args[0]) {
		return args[0]
	}
	return filepath.Join(cwd, args[0])
}

// options splits command arguments into positional values and flags.
// valued lists the flags that take a value; switches lists those that do not.
// A flag may be written anywhere, as "--name x" or "--name=x".
func options(args []string, valued, switches []string) (pos []string, flags map[string]string, err error) {
	flags = map[string]string{}
	has := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			pos = append(pos, arg)
			continue
		}
		name, value, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		switch {
		case has(switches, name) && !inline:
			flags[name] = "true"
		case has(valued, name) && inline:
			flags[name] = value
		case has(valued, name) && i+1 < len(args):
			i++
			flags[name] = args[i]
		case has(valued, name):
			return nil, nil, fmt.Errorf("%s needs a value", arg)
		default:
			return nil, nil, fmt.Errorf("unknown option %s", arg)
		}
	}
	return pos, flags, nil
}

// validate checks a definition and prints what it would run.
func (in invocation) validate(ctx context.Context) int {
	if len(in.args) > 1 {
		return in.usage()
	}
	path := definitionPath(in.cwd, in.args)
	def, err := pipeline.ParseFile(ctx, path)
	if err != nil {
		return in.fail(err)
	}
	if err := pipeline.Validate(def); err != nil {
		return in.fail(err)
	}
	plan, err := pipeline.Plan(def)
	if err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "%s is valid: pipeline %s, %d steps, concurrency %d\n", filepath.Base(path), plan.Name, len(plan.Graph), plan.Concurrency)
	for _, s := range def.Schedules {
		zone := "local time"
		if s.Timezone != "" {
			zone = s.Timezone
		}
		fmt.Fprintf(in.stdout, "schedule %s (%s, missed: %s)\n", s.Cron, zone, s.Missed)
	}
	for _, s := range plan.Graph {
		line := "step " + s.ID
		if len(s.Needs) > 0 {
			line += " needs " + strings.Join(s.Needs, ", ")
		}
		if len(s.Secrets) > 0 {
			line += " uses secrets " + strings.Join(s.Secrets, ", ")
		}
		fmt.Fprintln(in.stdout, line)
	}
	return 0
}
