# Command line reference

`truss` is one binary. Two commands work on their own; the rest talk to the
[daemon](daemon.md).

```text
truss <command> [arguments]
```

New to the command line? [Using the command line](cli-guide.md) is a tour with recipes. This page is
the reference.

Every command exits `0` on success and `1` on any error (`truss help exit-codes`). An error is a
line on standard error that starts with `error:`, sometimes followed by a `hint:` line with what to
do next:

```console
$ truss runs
error: the TRUSS daemon is not running
hint: start it with: truss daemon start
```

A mistyped command gets a suggestion, and wrong arguments show that command's usage instead of the
whole list:

```console
$ truss regster build
error: unknown command "regster"
hint: did you mean "register"?
usage: truss <command> [arguments]
Run "truss help" to see every command.

$ truss daemon restart
error: truss daemon has no "restart"; did you mean "start"?
usage: truss daemon start|stop|status|run|install|uninstall|logs
Run "truss help daemon" for details.
```

TRUSS prints no colour, so `NO_COLOR` and redirected output need no special care. Scripts should
read `--json`, not the tables: their wording and columns may change between minor versions.

## Start a project

| Command | What it does |
|---|---|
| `truss init [--yes] [--name <project>] [--flow <name>] [--template <t>]` | Sets up this folder: `truss.yaml` and a first flow in `.truss/flows/`. Asks what the flow does (a template suggested from `go.mod`, `package.json`, `pyproject.toml`, `requirements.txt`, `Makefile`), how it starts (by hand, a schedule in plain words or a cron expression, or when files change) and whether to notify on failure. `--yes`, or no terminal, takes every suggestion. Refuses a folder that has a `truss.yaml`; never overwrites a file. |
| `truss flows new [<name>] [--template <t>] [--yes]` | Adds a flow to this project with the same questions. |

Templates: `go`, `node`, `python`, `make`, `backup`, `shell`.

## Look around

Read-only commands that answer "what is going on?". They change nothing and never start or stop the
daemon. Most take `--json` for scripts ([Using the command line](cli-guide.md#scripting)).

| Command | What it does | Daemon |
|---|---|---|
| `truss status [--json]` | One screen: project, daemon, registered flows by state, latest run. Exits `0` whether or not a daemon runs. Never prints the Desktop token. | optional |
| `truss dashboard [--once]` | An ASCII dashboard of the daemon, load, flows and latest runs. See [the dashboard](cli-guide.md#the-dashboard). | optional |
| `truss project [show] [--json]` | The project of this folder: name, description, root, flows, how many are registered. | optional |
| `truss projects [list] [--json]` | The projects the daemon knows, from the files it has registered, with counts of flows, running, paused and invalid. | needed |
| `truss flows [list] [--json]` | The flows of this project with steps, schedules and whether the daemon has them (`registered`, `paused`, `not-registered`, or `unknown` without a daemon). A flow that does not load is listed with its error. | optional |
| `truss flows show <name\|path> [--json]` | One flow: concurrency, overlap, the **names** of environment variables and secrets (never values), schedules, and a table of its steps. | optional |
| `truss flows show <name\|path> <step> [--values] [--json]` | One step: its command, the steps before and after it, its settings, and a table of the environment it can use with where each value comes from. Secrets show as `set` or `missing`; token-like names show `***`. `--values` also shows the values of host variables the command reads. See [Debugging a step](cli-guide.md#debugging-a-step). | optional |
| `truss flows graph <name\|path>` | The steps in stages; steps in one stage can run together. | no |
| `truss pipelines show <name> [--json]` | One registered flow: state, file, project, schedules with next run, steps, latest runs. | needed |
| `truss schedules [list] [--json]` | Every schedule of every registered flow, soonest first. | needed |
| `truss env` | The TRUSS home and where it comes from, the registry, secrets file and daemon log paths, the project, and whether a daemon runs. No secret name or value. | optional |

`flows` reads files and `pipelines` asks the daemon: a flow is a file in `.truss/flows/`, a flow
is a file the daemon has registered ([glossary](GLOSSARY.md)).

```console
$ truss status
TRUSS      0.2.0 (linux/amd64)
Project    demo, 3 flows in .truss/flows/
Daemon     running, pid 4121, up 2h 5m, 127.0.0.1:7878
Desktop    http://127.0.0.1:7878/ (open it with: truss desktop)
Runs       1 running, 0 queued (limit 4 at once)
Flows      3 registered: 2 ready, 1 running, 0 paused, 0 invalid
Last run   build running just now (truss runs show 20261117-091500-a1b2c3)
Home       /home/me/.truss
```

## Help

`truss help` prints all commands. `truss help <command>` or `truss <command> --help` explains one
(for example `truss help flows`). `truss run --help` and `truss validate --help` are not
help: their argument is a flow name or a path, so a file may be named like a flag. `truss help <unknown>` is an unknown
command and exits `1`. `truss help exit-codes` lists the exit codes.

### Shell completion

`truss completion bash|zsh|fish|powershell` prints a script that completes commands, subcommands
and the flow names (`.yaml` and `.yml` files of `.truss/flows/`) of the project in the current folder. Load it once per shell, or add the line
to your shell profile:

| Shell | Line |
|---|---|
| bash | `eval "$(truss completion bash)"` |
| zsh | `eval "$(truss completion zsh)"` (it runs `compinit` if your profile has not) |
| fish | `truss completion fish \| source` |
| PowerShell | `truss completion powershell \| Out-String \| Invoke-Expression` |

## Without the daemon

| Command | What it does |
|---|---|
| `truss run <name\|path>` | Runs a flow right here and waits for it. With no argument it is an error: it lists the flows in `.truss/flows/` and exits `1`. |
| `truss run <name\|path> --track` | The same, and also reports the run to the daemon, which must be running. |
| `truss run <name\|path> --step <id> [--with-needs]` | A debug run: only that step. The steps it needs are not run and the step starts as if they had succeeded; `--with-needs` runs them too. The log is printed, and the saved `run.log` is not replaced. |
| `truss run <name\|path> --from <id>` | A debug run of the step and every step that needs it. Jobs after the step keep their `if`; only the job of the step you named loses it. |
| `truss run <name\|path> --notify` | Sends the flow's [notifications](pipeline-reference.md#notify-hear-how-a-run-ended) when the run ends, as the daemon would. A notification that fails is reported and does not change the exit code. |
| `truss run <name\|path> --env NAME=VALUE` | Sets or replaces an environment variable for the steps that run, above everything in the file. Repeatable; works with a whole run too. A secret cannot be set this way: use `truss secret set`. |
| `truss validate [<name\|path>]` | Checks one flow and prints its schedules and steps. With no argument, checks `truss.yaml` and every flow. Runs nothing. |
| `truss help`, `truss version` | Usage and version. |

A debug run does not combine with `--track`. `--step`, `--from`, `--with-needs` and `--env` are options of `run`, so a file named like one runs as `./--step`. When a `truss run` fails, it prints on standard error the command to run only the failed step (`to run only this step again: truss run <flow> --step <id>`), without your `--env` options. The command is printed only when the flow and the id are plain words: letters, digits and `_ . / : @ + = -` (and `\` on Windows), not starting with `-`, `@` or `=`.

`truss run` does not talk to the daemon, so the run does not appear in `truss runs` or in Desktop (add `--track`, below). It prints the run log as it happens and keeps the last one in `.truss/state/<name>/run.log`.
Interrupting it (Ctrl-C) ends every running step. Schedules in the file are ignored: `run`
means now. Steps start in the working directory, which is the project root when you run TRUSS from the project folder. The layout is in [Project layout](pipeline-reference.md#project-layout).

```console
$ truss run hello
2026-11-12 09:00:00 ==== Setup: the environment the steps run in ====
2026-11-12 09:00:00 setup environment ready
...
2026-11-12 09:00:00 ==== Step hello ====
2026-11-12 09:00:00 step hello started
2026-11-12 09:00:00 step hello | Hello from TRUSS
2026-11-12 09:00:00 step hello finished duration 0.004s exit 0
2026-11-12 09:00:00 ==== Summary ====
2026-11-12 09:00:00 summary succeeded steps 1 duration 0.006s
```

### Reading the run log

A line that starts with `====` after the time is a heading. It opens the setup (the
machine the steps run on), a step, or the summary. Everything a step prints is labelled
`step <id> |`, so what TRUSS says and what your commands print cannot be mixed up, even when
steps run at the same time. Desktop shows the same text as one card per step
([Desktop](desktop.md#the-run-page)). A step that is retried gets a heading for each attempt.

### Flow arguments

`run`, `validate` and `register` take a flow as a name or a path:

- An argument that contains `/` or `\`, or ends in `.yaml` or `.yml`, is a **path**, relative to
  the working directory. `truss run ./ci/build.yaml` runs that file.
- Any other argument is a **name**: the file name in `.truss/flows/` without its extension.
  `truss run build` runs `.truss/flows/build.yaml`.
- A name that is not found lists the available flows. TRUSS never tries the other reading.
- `truss run` with no argument is an error. It lists the flows and exits `1`.
- `truss run truss.yaml` is an error: `truss.yaml` is the project file, not a flow. A
  `truss.yaml` in another folder, given by path, runs like any other flow file.
- A command that needs a project and finds no `truss.yaml` in the working directory names the
  folder it looked in. TRUSS never looks in parent folders.

## Secrets

Secrets need no daemon. The value is never an argument, so it stays out of your shell history
and the process list.

| Command | What it does |
|---|---|
| `truss secret set <name>` | Stores a secret. On a terminal it asks twice, with no echo. From a pipe or file the whole input is the value, minus one trailing newline. |
| `truss secret list` | Prints the names, one per line. Never values. |
| `truss secret remove <name>` | Deletes a secret and prints `secret <name> removed`. An unknown name is an error. |

```console
$ printf %s "$NPM_TOKEN" | truss secret set NPM_TOKEN
secret NPM_TOKEN saved
```

Secrets live in `secrets.json` in the [TRUSS home](daemon.md#where-state-lives). They are **not
encrypted**. On Linux and macOS the file is `0600` and TRUSS refuses to read it if others can.
On Windows there are no mode bits: the file relies on the permissions it inherits from your user
profile, and TRUSS does not change them; if you point `TRUSS_HOME` at a shared folder, nothing
checks it. On systems other than Linux, macOS and Windows, and when standard input is not a console
(some Windows terminals such as Git Bash), `set` reads the value from standard input with no prompt. The daemon has no API for secrets. Flows use them
through [`secrets`](pipeline-reference.md#secrets).

## The daemon

| Command | What it does |
|---|---|
| `truss daemon start [--address <host:port>] [--max-runs <n>] [--history <n>]` | Starts the background service and returns. The flags win over `config.yaml` for this start. |
| `truss daemon stop [--wait]` | Stops it. Active runs are cancelled: each step is asked to stop and killed after its `kill_timeout`. `--wait` first waits until no run is running or queued; a run a schedule starts meanwhile is waited for too, and Ctrl-C ends the wait. |
| `truss daemon status [--json]` | Says whether it runs, where, how busy it is, and whether it starts at login. Exits `1` when it does not run. `--json` prints `{"running": ..., "daemon": ...}` and keeps the exit code. |
| `truss daemon logs [-n <lines>] [--follow]` | Prints the last 50 lines (or `-n`) of the daemon's own log, and with `--follow` keeps printing until Ctrl-C. Reads a file, so it works when the daemon is stopped. |
| `truss daemon config [--json]` | Shows each daemon setting (`address`, `max_runs`, `history`), its value and where it comes from: a `TRUSS_DAEMON_*` variable, `config.yaml` or the default. See [Configuration](daemon.md#configuration). |
| `truss daemon run` | Runs the service in the terminal, logging to standard error. For debugging and for service managers. |
| `truss daemon install` | Makes the daemon start at login: a systemd user unit (Linux), a launchd agent (macOS) or a Task Scheduler task (Windows). Needs no root or administrator rights. Refuses a binary in a temporary directory. See [Starting at login](daemon.md#starting-at-login). |
| `truss daemon uninstall` | Removes what `install` created, and nothing else. |
| `truss desktop` | Opens [TRUSS Desktop](desktop.md). `--no-open` only prints the address. `--demo` needs no daemon and shows sample data, for trying or reviewing the design; Ctrl-C stops it. |

Options of `daemon start`, `daemon run` and `daemon install`:

| Option | Default | Meaning |
|---|---|---|
| `--address <host:port>` | `127.0.0.1:7878` | Where to listen. Loopback only. Port `0` picks a free port. |
| `--max-runs <n>` | `4` | How many runs may execute at the same time. |

### Tracking a local run

`truss run --track` still runs in your shell, with your environment and working directory, and
prints the same run log. It also reports to the daemon, so the run shows up in `truss runs` and
Desktop with the trigger `local`, its steps, and its log:

```console
$ truss run --track
tracking as run 20261112-090000-a1b2c3 (truss logs 20261112-090000-a1b2c3)
2026-11-12 09:00:00 ==== Setup: the environment the steps run in ====
...
$ truss runs
RUN                     FLOW         STATUS     TRIGGER  CREATED              DURATION  NOTE
20261112-090000-a1b2c3  hello-world  succeeded  local    2026-11-12 09:00:00  0.1s
```

- `--track` needs the daemon. Without one it exits `1` with `start it with: truss daemon start`
  and runs nothing. The CLI never starts the daemon.
- The flow's output and exit code do not depend on tracking. If the daemon goes away while the
  run executes, you get one `warning:` on stderr and the run finishes normally.
- Cancelling the run in Desktop or with `truss cancel <run-id>` stops it: it exits `1` with
  `error: cancelled from the daemon`.
- The id is printed on stderr, so stdout stays the run log.

## Registered flows

A command that takes a registered flow's name accepts the name its file gives it now or the name
it was registered under; they differ after a rename in the file (see [Registering
flows](daemon.md#registering-flows)).

| Command | What it does |
|---|---|
| `truss register <name> [--name <name>]`, `truss register <path> [--name <name>]`, `truss register --all` | Tells the daemon about a flow. The name defaults to `flow.name`, or the file name. `--all` registers every flow in the project, each under its flow name; `--name` is not allowed with `--all`. With no argument it is an error that lists the flows. When every flow registered, `--all` prints `registered N flows (M with schedules)` (`registered 1 flow (M with schedules)` for one). It prints no summary when a flow failed. |
| `truss unregister <name>` | Forgets it. The file and the run history stay. |
| `truss pipelines [list] [--json]` | Lists registrations with state, next scheduled run and last run. `--json` prints the daemon's objects. See also `pipelines show`, above. |
| `truss pause <name>` | Stops the flow's schedules. Manual runs still work. |
| `truss resume <name>` | Restarts them, counting from now. |

## Runs

| Command | What it does |
|---|---|
| `truss trigger <name>` | Requests a run and prints its id. |
| `truss trigger <name> --follow` | Requests a run, prints its log live, and exits `0` only if it succeeded. |
| `truss runs [<name>]` | Lists the newest runs, optionally of one flow. |
| `truss runs [<name>] --status <s> --active --limit <n> --json` | Filters the list. `<s>` is `queued`, `running`, `succeeded`, `failed`, `cancelled` or `skipped`; `--active` means queued or running; the default limit is 30. |
| `truss runs show <run-id\|last> [--flow <name>] [--json]` | One run: status, reason, trigger, times, each step's status, exit code, time and attempt, and a timeline. `last` is the newest run (of `--flow` if given; `--pipeline` is its older name). Exits `0` even for a failed run: it answers a question. |
| `truss logs <run-id>` | Prints a run log. `--follow` keeps printing until the run ends. |
| `truss cancel <run-id>` | Ends a queued or running run. |
| `truss cache clear <name>` | Empties the flow's cache folder. Needs no daemon. For a daemon run, see [the daemon page](daemon.md#data-and-cache-folders). |

A flow may be called `show`: `truss runs show` alone still lists its runs, and only `truss runs show <something>`
shows a run.

`-f` is short for `--follow`, and `-n` for `--limit` in `truss runs`. Interrupting a followed log stops the printing, not the run.

## Environment

| Variable | Meaning |
|---|---|
| `TRUSS_HOME` | Where the daemon keeps its state. Default: `.truss` in your home directory. |
| `TRUSS_DAEMON_ADDRESS`, `TRUSS_DAEMON_MAX_RUNS`, `TRUSS_DAEMON_HISTORY` | Daemon settings; they win over `config.yaml` and lose to a flag. See [Configuration](daemon.md#configuration). |
