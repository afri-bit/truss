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

`truss run` prints the run log as it happens and keeps the last one in `.truss/run.log`.
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
