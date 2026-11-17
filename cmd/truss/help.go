package main

import "fmt"

// helpTopics holds `truss help <command>`. A command also prints its topic
// for `truss <command> --help`, except run, where any argument is a file name.
var helpTopics = map[string]string{
	"status": `truss status [--json]

One screen that answers "is everything fine?": the project of this folder, the
daemon, the registered flows and the latest run. Exits 0 whether or not a daemon
runs. --json prints the same facts for a script.
`,
	"dashboard": `truss dashboard [--once]

An ASCII dashboard: daemon, load, flows and the latest runs, in frames. On a
terminal it redraws every two seconds until Ctrl-C. --once, a pipe, a file and the
Windows console draw it once. The width follows COLUMNS (60 to 120, default 80).
Needs no colour and no special font.
`,
	"project": `truss project [show] [--json]

The project of this folder: its name, description, root, how many flows it holds
and, when the daemon runs, how many of them are registered. Needs a truss.yaml in
this folder.
`,
	"projects": `truss projects [list] [--json]

The projects the daemon knows, grouped from the files of its registrations: how
many flows each has and how many run, are paused or are broken. Needs the daemon.
`,
	"flows": `truss flows [list] [--json]
truss flows show <name|path> [--json]
truss flows graph <name|path>

Read the pipeline files of the project. Nothing runs and the daemon is optional.
  list    every flow with its steps, schedules and whether the daemon has it
  show    one flow: concurrency, overlap, environment and secret names (never
          values), schedules and a table of its steps
  graph   the order the steps run in, as stages; steps of a stage can run together
`,
	"pipelines": `truss pipelines [list] [--json]
truss pipelines show <name> [--json]

The pipelines registered with the daemon. show adds the file, the project, the
schedules with their next run, the steps and the latest runs. Needs the daemon.
`,
	"schedules": `truss schedules [list] [--json]

Every schedule of every registered pipeline, soonest first. Needs the daemon.
`,
	"runs": `truss runs [<pipeline>] [--status <s>] [--active] [--limit <n>] [--json]
truss runs show <run-id|last> [--pipeline <name>] [--json]

List runs, newest first, or show one: its status, each step's exit code and time,
and a timeline of when the steps ran. "last" is the newest run, of one pipeline
with --pipeline. --status is one of queued, running, succeeded, failed, cancelled,
skipped. --active means queued or running. Needs the daemon.
`,
	"daemon": `truss daemon start|stop|status|run|install|uninstall|logs

  start [--address <host:port>] [--max-runs <n>]   start the service in the background
  stop                                            stop it; active runs are cancelled
  status [--json]                                 exits 1 when it is not running
  run                                             run it in this terminal
  install | uninstall                             start it at login, or stop doing so
  logs [-n <lines>] [--follow]                    the daemon's own log file; works when it is stopped
`,
	"env": `truss env

Where TRUSS keeps its files and what it sees: the version, the system, the TRUSS
home (and whether TRUSS_HOME set it), the registry, the secrets file, the daemon
log, this folder's project and whether the daemon runs. It prints no secret names
and no values.
`,
	"run": `truss run <name|path> [--track]

Run a pipeline here, without the daemon. --track also reports it to the daemon.
`,
	"validate": `truss validate [<name|path>]

Check truss.yaml and every pipeline, or one pipeline, without running it.
`,
	"secret": `truss secret set|list|remove <name>

Store a secret (the value is read from standard input), list names, or remove one.
`,
	"register": `truss register <name|path> [--name <name>]
truss register --all

Make a pipeline known to the daemon. Related: unregister, pause, resume.
`,
	"trigger": `truss trigger <name> [--follow]

Start a run now. --follow prints its log and exits 0 only if it succeeded.
`,
	"logs": `truss logs <run-id> [--follow]

Print a run's log. For the daemon's own log, use: truss daemon logs
`,
	"cancel": `truss cancel <run-id>

End a queued or running run.
`,
	"desktop": `truss desktop [--no-open] [--demo]

Open TRUSS Desktop. --no-open prints the address only; --demo shows sample data.
`,
	"cache": `truss cache clear <name>

Remove a pipeline's cache folder; the next run recreates it. Needs no daemon.
`,
}

// helpAliases send commands without a topic of their own to one that has.
var helpAliases = map[string]string{
	"unregister": "register", "pause": "register", "resume": "register",
}

// help prints the usage, or the topic of one command, on standard output.
func (in invocation) help(topic []string) int {
	if len(topic) == 0 {
		fmt.Fprint(in.stdout, usageText)
		return 0
	}
	// An unknown topic, or more than one word, stays a usage error: that is
	// what `truss help extra` did before topics existed (spec 002).
	if len(topic) != 1 {
		return in.usage()
	}
	key := topic[0]
	if a, ok := helpAliases[key]; ok {
		key = a
	}
	text, ok := helpTopics[key]
	if !ok {
		return in.usage()
	}
	fmt.Fprint(in.stdout, text)
	return 0
}

// wantsHelp reports whether args are "<command> --help" for a command that has
// a topic. run is excluded: any argument after it is a file name.
func wantsHelp(args []string) bool {
	if len(args) != 2 || args[0] == "run" || (args[1] != "--help" && args[1] != "-h") {
		return false
	}
	if a, ok := helpAliases[args[0]]; ok {
		args = []string{a}
	}
	_, ok := helpTopics[args[0]]
	return ok
}
