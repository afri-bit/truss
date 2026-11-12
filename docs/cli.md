# Command line reference

`truss` is one binary. Two commands work on their own; the rest talk to the
[daemon](daemon.md).

```text
truss <command> [arguments]
```

Every command exits `0` on success and `1` on any error. Errors are one line on standard error
that starts with `error:`. An unknown command or wrong arguments print the usage text and exit `1`.

## Without the daemon

| Command | What it does |
|---|---|
| `truss run [<path>]` | Runs a pipeline right here and waits for it. Default file: `truss.yaml`. |
| `truss validate [<path>]` | Checks a file and prints its schedules and steps. Runs nothing. |
| `truss help`, `truss version` | Usage and version. |

`truss run` never talks to the daemon, so the run does not appear in `truss runs` or in Desktop. It prints the run log as it happens and keeps the last one in `.truss/run.log`.
Interrupting it (Ctrl-C) ends every running step. Schedules in the file are ignored: `run`
means now.

```console
$ truss run
2026-11-12 09:00:00 setup environment ready
...
2026-11-12 09:00:00 step hello started
2026-11-12 09:00:00 step hello | Hello from TRUSS
2026-11-12 09:00:00 step hello finished duration 0.004s exit 0
2026-11-12 09:00:00 summary succeeded steps 1 duration 0.006s
```

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
(some Windows terminals such as Git Bash), `set` reads the value from standard input with no prompt. The daemon has no API for secrets. Pipelines use them
through [`secrets`](pipeline-reference.md#secrets).

## The daemon

| Command | What it does |
|---|---|
| `truss daemon start` | Starts the background service and returns. |
| `truss daemon stop` | Stops it. Active runs are cancelled. |
| `truss daemon status` | Says whether it runs, where, and how busy it is. Exits `1` when it does not run. |
| `truss daemon run` | Runs the service in the terminal, logging to standard error. For debugging and for service managers. |
| `truss desktop` | Opens [TRUSS Desktop](desktop.md). `--no-open` only prints the address. |

Options of `daemon start` and `daemon run`:

| Option | Default | Meaning |
|---|---|---|
| `--address <host:port>` | `127.0.0.1:7878` | Where to listen. Loopback only. Port `0` picks a free port. |
| `--max-runs <n>` | `4` | How many runs may execute at the same time. |

## Registered pipelines

| Command | What it does |
|---|---|
| `truss register [<path>] [--name <name>]` | Tells the daemon about a file. The name defaults to `pipeline.name`. |
| `truss unregister <name>` | Forgets it. The file and the run history stay. |
| `truss pipelines` | Lists registrations with state, next scheduled run and last run. |
| `truss pause <name>` | Stops the pipeline's schedules. Manual runs still work. |
| `truss resume <name>` | Restarts them, counting from now. |

## Runs

| Command | What it does |
|---|---|
| `truss trigger <name>` | Requests a run and prints its id. |
| `truss trigger <name> --follow` | Requests a run, prints its log live, and exits `0` only if it succeeded. |
| `truss runs [<name>]` | Lists the newest runs, optionally of one pipeline. |
| `truss logs <run-id>` | Prints a run log. `--follow` keeps printing until the run ends. |
| `truss cancel <run-id>` | Ends a queued or running run. |

`-f` is short for `--follow`. Interrupting a followed log stops the printing, not the run.

## Environment

| Variable | Meaning |
|---|---|
| `TRUSS_HOME` | Where the daemon keeps its state. Default: `.truss` in your home directory. |
