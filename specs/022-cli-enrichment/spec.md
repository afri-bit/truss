# Feature Specification: A richer, layered command line

**Spec**: `022-cli-enrichment` | **Created**: 2026-11-17 | **Status**: Implemented

Extends [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md),
[`016-project-layout`](../016-project-layout/spec.md) and
[`002-local-run-log`](../002-local-run-log/spec.md) (help). User docs:
[`docs/cli.md`](../../docs/cli.md) (reference) and [`docs/cli-guide.md`](../../docs/cli-guide.md) (tour).
Decision: [ADR 0019](../../docs/decisions/0019-the-cli-grows-by-read-only-commands-and-ascii-screens.md).
Risk class: tier 2 (CLI output and usage are public contracts). Scope set by the maintainer on
2026-11-17: ask about status, projects, flows, environment, the daemon and Desktop from the
terminal; layered commands; a small ASCII screen; docs on the site.

## User stories

1. **Is everything fine?** (P1). One command tells me whether the daemon runs, how busy it is, how
   many flows are registered and how the last run went. Without a daemon it says so and still tells
   me about the project in this folder.
2. **What is in my project?** (P1). I list the flows of a project, see one flow's schedules,
   environment variable names, secrets used and steps, and see the order its steps run in.
3. **How did that run go?** (P1). I show one run (or the latest) with each step's status, exit code,
   time and a timeline, and I filter the run list by status.
4. **What does the daemon know?** (P2). I list its projects and every schedule soonest first, show
   one registered pipeline, read the daemon's own log, and get everything as JSON for a script.
5. **Watch it** (P2). An ASCII dashboard shows the daemon, load, flows and latest runs, live in a
   terminal.
6. **Find my way** (P2). `truss help <command>` and `truss <command> --help` explain a command.

## Requirements

**Layers.** Commands are layered: a noun, then what to do with it. The commands of v0.1.0 keep
working with the same output, flags and exit codes (see Compatibility).

- **FR-001** `truss status [--json]` prints the version and system, the project of the working
  directory (or says there is none), and the daemon: running (pid, uptime, address, Desktop address,
  runs running and queued, the limit), or not running. With a daemon it also prints the registered
  flows by state (ready, running, paused, invalid) and the latest run. It exits `0` with or without a
  daemon. It never prints the Desktop token. `--json` prints `version`, `project` (or `null`),
  `daemon` (the daemon status or `null`), `desktop_url` (`null` without a daemon), `pipelines` (counts),
  `recent_runs` (at most 5) and `home`. A problem is part of the answer, not a failure: a
  `truss.yaml` that cannot be read shows as `Project    unreadable: <reason>` (JSON: `project` is
  `null` and `project_error` says why), and a daemon that is registered but does not answer shows
  as `not answering: <reason>` (`daemon_error`). `status` still exits `0`.
- **FR-002** `truss dashboard [--once]` draws frames of plain ASCII: daemon, runs against the limit,
  host CPU and memory when the daemon reports them, the registered flows (state, next run, last run)
  and the latest runs. Every line of a frame has the same width: `COLUMNS` limited to 60 to 120,
  default 80. When standard output is a terminal on Linux or macOS and `--once` is absent, it
  redraws every two seconds until interrupted and exits `0`; otherwise (a pipe, a file, the Windows
  console, `--once`) it draws once. Without a daemon the frame says so and lists the project's flows.
- **FR-003** `truss project [show] [--json]` prints the project name, description, root, how many
  flows it has and, when a daemon runs, how many are registered. It needs a `truss.yaml` in the
  working directory. `truss projects [list] [--json]` lists the projects the daemon knows, grouped
  by the project folder of each registration's file (`.truss/flows/<file>` under a folder with a
  `truss.yaml`); registrations of other files are grouped as `(no project)`. Each group has its
  registered, running, paused and invalid counts. It needs the daemon.
- **FR-004** `truss flows [list] [--json]` lists the flows of the project with their step and
  schedule counts and whether the daemon has them: `registered`, `paused`, `not-registered`, or
  `unknown` when no daemon runs. It needs no daemon. A flow that does not load is listed with its
  error and does not hide the others. `truss flows show <name|path> [--json]` prints the pipeline
  name, file, project, registration, concurrency, overlap, the **names** of the environment
  variables and secrets its steps receive (never their values) (spec 023 shows the values of one
  step, under rules), its schedules, and a table of its
  steps (needs, timeout, retries, `if`, first line of `run`). `truss flows graph <name|path>` prints the steps in stages: a step is in
  the stage after the latest of the steps it needs, and each step lists what it needs. `show` of a flow that does not load is an error: exit `1`, and with `--json` the document
  (`valid: false`, `error`) is printed first. A `run` of more than one line shows its first line and
  `(+N lines)`, so a table never hides that more follows; `--json` has the whole text.
- **FR-005** `truss pipelines [list] [--json]` lists registered pipelines as before; `--json` prints
  the daemon's pipeline objects. `truss pipelines show <name> [--json]` prints one: its state, file,
  project, concurrency, overlap, registration time, active runs, schedules with their next run,
  steps and its five latest runs. Either name of a renamed flow finds it; when one pipeline's name is another's registered name, the exact name wins. An unknown name is an error.
- **FR-006** `truss schedules [list] [--json]` lists every schedule of every registered pipeline,
  soonest next run first (schedules with no next run last), with zone, missed policy and whether the
  pipeline is paused.
- **FR-007** `truss runs` takes `--status <s>` (`queued`, `running`, `succeeded`, `failed`,
  `cancelled`, `skipped`), `--active` (queued or running), `--limit <n>` (default 30) and `--json`,
  with or without a pipeline name. `--status` and `--active` together, a status that is not one of
  those, and a limit that is not a whole number of at least 1 are errors. A filter that matches
  nothing prints `no runs match`. A filter looks at the newest 1000 runs.
- **FR-008** `truss runs show <run-id|last> [--pipeline <name>] [--json]` prints one run: id,
  pipeline, status and reason, trigger, created time, duration, a count of steps by status, and a
  table of steps (status, exit code, time, attempt) with a timeline of when each ran inside the run.
  Step errors and skip reasons follow as notes, one line each. `last` is the newest run, of one
  pipeline with `--pipeline`. An unknown id is an error. The command is a query: it exits `0` for a
  failed run.
- **FR-009** `truss daemon logs [-n <lines>|--lines <n>] [--follow|-f]` prints the last lines
  (default 50) of the daemon's own log file. It needs no daemon. With no log file it says so and
  exits `0` (`1` with `--follow`). `--follow` prints what is appended until interrupted, then exits
  `0`. `truss daemon status --json` prints `{"running": bool, "daemon": status|null}` and, like the
  text form, exits `1` when the daemon does not run.
- **FR-010** `truss env` prints the version, the system, the TRUSS home and whether `TRUSS_HOME` set
  it, the paths of the registry, the secrets file and the daemon log (and whether they exist yet),
  the project of the working directory and whether a daemon runs. It prints no secret name or value.
- **FR-011** `truss help <command>` prints that command's help on standard output and exits `0`;
  so does `truss <command> --help` or `-h`, except for `run` and `validate`, whose argument is a
  flow name or a path, so `--help` is still read as one (spec 001, spec 016). `unregister`, `pause` and `resume` show the help of `register`. An unknown
  topic, or more than one word after `help`, is the usage error it always was.
- **FR-012** `--json` writes one JSON document, indented, on standard output. Daemon objects keep
  the field names of [the API](../../docs/daemon.md); the shapes of the new documents are in
  [`docs/cli-guide.md`](../../docs/cli-guide.md#scripting). Empty lists are `[]`, never `null`.
- **FR-013** Every new command works on Linux, macOS and Windows. The screens use ASCII only,
  no colour and no cursor movement, except the live dashboard's redraw on a Linux or macOS terminal.
  The CLI never starts or stops the daemon, and never changes a file.

- **FR-014** A text screen never passes a control character from a file or the daemon to the
  terminal (an escape, bell or carriage return, a C1 code, or an invisible format character such as a
  bidirectional override): each becomes `?`, a tab becomes a
  space. This covers the pipeline name, step ids, `needs`, `run` text, project description and
  errors, in the new commands and in `validate`. JSON output is not changed: it escapes. `daemon
  logs` reads only a regular file (never a named pipe, which would block for good), and `--follow`
  follows a log file that was replaced.

## Not in this spec

Shell completion, colour, a full-screen interactive UI, a `watch` for a single run (the dashboard
and `logs --follow` cover it), editing flows or schedules from the CLI, and a JSON form of
`validate`.

## Compatibility

Additive. A command, flag or output of v0.1.0 is unchanged, with these precise rules so a script
never changes meaning:

- A new word (`show`, `list`, `logs`, a group name) is taken only where the old command rejected
  the argument as usage, with one exception that cannot break anyone: `runs show` with nothing after
  it still lists the runs of a pipeline named `show`.
- `help <x>` for anything that is not a command topic is still the usage error.
- `runs <name>` and `pipelines` print the same table. New flags are only added.
- `truss status` and the other new top-level names were unknown commands (usage, exit `1`) before.

## Testing

`cmd/truss/overview_test.go` and `cmd/truss/ui_test.go` drive the commands against an in-process
daemon, including the exit codes, the compatibility rules, the width of every dashboard line and
that environment values never appear. `.github/scripts/e2e-test.sh` runs the built binary through
`status`, `flows`, `help` and `daemon status --json`.
