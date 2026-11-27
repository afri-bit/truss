package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
)

// usageText is the CLI usage printed for help and for unknown invocations.
const usageText = `TRUSS runs flows on your machine.

usage: truss <command> [arguments]

Look around (read-only; most take --json):
  status                              one screen: project, daemon, flows, latest run
  dashboard [--once]                  ASCII dashboard, live on a terminal
  project                             the project of this folder
  projects                            projects known to the daemon (needs it)
  flows [list]                        the flows of the project, and whether the daemon has them
  flows show <name|path>              one flow: schedules, environment names, steps
  flows show <name|path> <step>       one step: command, needs, and the environment it can use
  flows graph <name|path>             the stages a flow runs its steps in
  pipelines show <name>               one registered flow with its latest runs
  schedules                           every schedule, soonest first (needs the daemon)
  runs show <run-id|last>             one run: steps, exit codes, timeline
  env                                 where TRUSS keeps its files; what it sees

Run here, without the daemon:
  run <name|path> [--track]           run a flow: a name from .truss/flows/ or a file path;
                                      --track also reports it to the daemon (needs it)
  run <name|path> --step <id> | --from <id>
                                      debug: run one step, or a step and what needs it;
                                      add --with-needs to --step for what it needs; the run
                                      log is not saved. --env NAME=VALUE overrides a variable
  validate [<name|path>]              check truss.yaml and every flow, or one flow,
                                      without running it

The daemon:
  daemon start [--address <host:port>] [--max-runs <n>] [--history <n>]
  daemon stop [--wait] | status [--json]
                                      manage the background service; --wait lets runs finish
  daemon logs [-n <lines>] [--follow] the daemon's own log file
  daemon config [--json]              each daemon setting and where it comes from
  daemon run                          run the service in this terminal
  daemon install [--address <host:port>] [--max-runs <n>]
  daemon uninstall                    start the daemon at login, or stop doing so
  desktop [--no-open] [--demo]        open TRUSS Desktop in the browser
                                      --demo shows sample data and needs no daemon

Secrets (no daemon needed; values come from standard input, never an argument):
  secret set <name>                   store a secret
  secret list                         list secret names
  secret remove <name>                delete a secret

Registered flows (these need the daemon):
  register <name|path> [--name <name>]  make a flow known to the daemon
  register --all                      register every flow in .truss/flows/
  unregister <name>                   forget it; its run history stays
  pipelines                           list registered flows
  pause <name> | resume <name>        stop or restart a flow's schedules
  trigger <name> [--follow]           start a run now
  runs [<name>] [--status <s>] [--active] [-n <n>]
                                      list runs, newest first
  logs <run-id> [--follow]            show a run log
  cancel <run-id>                     end a queued or running run

Files only (no daemon needed):
  cache clear <name>                  remove a flow's cache folder; the next run recreates it

  completion bash|zsh|fish|powershell print a shell completion script
  help [<command>] | version          truss help <command> explains one command;
                                      truss help exit-codes lists the exit codes
`

// invocation is everything one CLI call may use. Commands write only to
// stdout and stderr and return the process exit code.
type invocation struct {
	// command is the first word, set once it is a known command; usage
	// errors then show that command's usage instead of the whole list.
	command string
	args    []string
	cwd     string
	env     []string
	stdin   io.Reader // nil reads as empty
	stdout  io.Writer
	stderr  io.Writer
}

// fail prints an error and returns exit code 1. Advice at the end of the
// message, "(start it with: ...)", goes on a hint line of its own.
func (in invocation) fail(err error) int {
	printError(in.stderr, err.Error())
	return 1
}

// printError writes an error line, and a hint line when the message ends
// with advice (spec 025).
func printError(w io.Writer, msg string) {
	text, hint := splitHint(msg)
	fmt.Fprintf(w, "error: %s\n", text)
	if hint != "" {
		fmt.Fprintf(w, "hint: %s\n", hint)
	}
}

// usage reports wrong arguments and returns exit code 1: the command's own
// usage when it has one, else the whole list.
func (in invocation) usage() int {
	if in.command != "" && in.commandUsage(in.command) {
		return 1
	}
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
	case len(args) >= 2 && args[0] == "help":
		return in.help(args[1:])
	case wantsHelp(args):
		return in.help(args[:1])
	case isVersion(args):
		fmt.Fprintln(stdout, Version)
		return 0
	case len(args) == 0:
		return in.usage()
	}
	in.args = args[1:]
	if contains(commands, args[0]) {
		in.command = args[0]
	}
	switch args[0] {
	case "run":
		// --track, --with-needs, --step, --from and --env are the options of
		// run; any other argument is the path, so a file named like a flag
		// still runs (spec 001, spec 023).
		pos, opts, err := parseRunArgs(in.args)
		if err != nil {
			return in.fail(err)
		}
		if len(pos) > 1 {
			return in.usage()
		}
		return runLocal(ctx, pos, opts, cwd, env, stdout, stderr)
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
	case "status":
		return in.status(ctx)
	case "dashboard":
		return in.dashboard(ctx)
	case "project":
		return in.projectCmd(ctx)
	case "projects":
		return in.projects(ctx)
	case "flows":
		return in.flows(ctx)
	case "schedules":
		return in.schedules(ctx)
	case "env":
		return in.environment(ctx)
	case "completion":
		return in.completion()
	case "version":
		// version with extra words is a usage error (spec 002); help with
		// extra words is handled by in.help.
		fmt.Fprint(stderr, usageText)
		return 1
	}
	if strings.HasPrefix(args[0], "-") {
		return in.usage()
	}
	return in.unknownCommand(args[0])
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

// validate checks the project file and every pipeline, or one pipeline, and
// prints what each would run (spec 016 FR-011).
func (in invocation) validate(ctx context.Context) int {
	if len(in.args) > 1 {
		return in.usage()
	}
	if len(in.args) == 0 {
		return in.validateProject(ctx)
	}
	if isProjectFile(in.cwd, in.args[0]) {
		return in.validateMetadata(project.FileName)
	}
	file, err := resolveFile(in.cwd, in.args[0])
	if err != nil {
		return in.fail(err)
	}
	if err := in.validateFile(ctx, file); err != nil {
		return in.fail(err)
	}
	return 0
}

// validateMetadata checks truss.yaml of the working directory.
func (in invocation) validateMetadata(display string) int {
	proj, err := project.Open(workingRoot(in.cwd))
	if err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "%s is valid: project %s\n", display, proj.Name)
	return 0
}

// validateProject checks truss.yaml and every flow. A broken flow is reported
// and does not stop the others; the exit code is 1 when any failed.
func (in invocation) validateProject(ctx context.Context) int {
	proj, err := project.Open(workingRoot(in.cwd))
	if err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "%s is valid: project %s\n", project.FileName, proj.Name)
	flows, err := proj.Flows()
	if err != nil {
		return in.fail(err)
	}
	if len(flows) == 0 {
		fmt.Fprintln(in.stdout, "no flows in .truss/flows/")
		return 0
	}
	code := 0
	for _, f := range flows {
		file := pipelineFile{path: f.Path, display: ".truss/flows/" + f.File, stem: f.Name}
		if err := in.validateFile(ctx, file); err != nil {
			fmt.Fprintf(in.stderr, "error: %s: %s\n", file.display, err.Error())
			code = 1
		}
	}
	return code
}

// validateFile checks one pipeline file and prints its result.
func (in invocation) validateFile(ctx context.Context, file pipelineFile) error {
	l, err := load(ctx, file)
	if err != nil {
		return err
	}
	plan, err := pipeline.Plan(l.def)
	if err != nil {
		return err
	}
	fmt.Fprintf(in.stdout, "%s is valid: flow %s, %d steps, concurrency %d\n", clean(file.display), clean(plan.Name), len(plan.Graph), plan.Concurrency)
	if l.declared != "" && l.declared != file.stem {
		fmt.Fprintf(in.stdout, "note: %s declares the flow name %q, not %q; the run log and the daemon use %q\n", clean(file.display), l.declared, file.stem, l.declared)
	}
	for _, s := range l.def.Schedules {
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
		fmt.Fprintln(in.stdout, clean(line))
	}
	return nil
}
