# How a run works

What happens between `truss run build` (or a schedule firing) and the last line of the log, and why
TRUSS behaves the way it does.

## Processes

```text
truss (the CLI, or the daemon)        one process; the flow is planned and run inside it
├── step "test"   → sh -c "go test ./..."   a child process in a process group of its own
│                   └── go, compilers, ...  everything it starts stays in that group
└── step "lint"   → sh -c "golangci-lint run"
```

- **A run is not a process of its own.** The flow is read, checked and planned inside the `truss`
  process that starts it: the CLI for `truss run`, the daemon for a schedule or `truss trigger`.
- **Each step is a child process**: your shell (`sh -c` on Linux and macOS, `cmd /C` on Windows)
  running the step's `run` text. Steps that do not need each other run side by side, up to the
  flow's `concurrency`.
- **Each step has its own process tree**: a process group on Linux and macOS, a job object on
  Windows. Whatever the step starts, even in the background, belongs to it and ends with it.
- When a step's shell exits, TRUSS ends what is left of its tree, so a step never leaves a process
  running behind it.

## Order and failure

- A step starts when every step it `needs` has succeeded; steps of a job run in order.
- The first failure that is not `continue-on-error` stops the run: running steps are stopped and
  nothing new starts, except steps whose `if` asks for `failure()` or `always()`.
- A step can be tried again (`retries`, `retry_delay`); its result is its last attempt.

## Stopping a step

A step is stopped when its `timeout` passes, when the run is cancelled (Ctrl-C, `truss cancel`),
when another step fails, or when the daemon stops.

- **Linux and macOS:** the whole process group gets `SIGTERM`, so scripts can clean up; whatever is
  still running after `kill_timeout` (default 10 seconds, at most one minute) gets `SIGKILL`.
- **Windows:** the job object is ended at once.
- A stopped step counts as failed, whatever its exit code. A second Ctrl-C kills at once.

## The environment of a step

A step's environment is built from layers. A later layer wins over an earlier one:

1. the environment TRUSS started in (your shell, or the daemon's),
2. the [built-in variables](pipeline-reference.md#built-in-variables) (`TRUSS_FLOW`,
   `TRUSS_STEP`, `TRUSS_RUN_ID`, `TRUSS_PROJECT_DIR`, `TRUSS_TEMP_DIR`, ...),
3. `env` in `truss.yaml` (the project),
4. `env` of the flow,
5. `env` of the job,
6. `env` of the step,
7. the step's `secrets`, from the secret store,
8. `--env NAME=VALUE` of a debug run.

`${NAME}` in a value is replaced when the step starts, from the layers below it. Each step is a new
process, so a variable a step `export`s does not reach the next step; put shared values in `env`.
See [variables and secrets](guide-variables.md).

## Where things are kept

| What | Where |
|---|---|
| The log of a local run | `.truss/state/<flow>/run.log` in the project |
| A folder that stays between runs (`TRUSS_DATA_DIR`) | `.truss/state/<flow>/data/` locally, `~/.truss/pipelines/<flow>/` for the daemon |
| A folder for one run (`TRUSS_TEMP_DIR`) | the system's temporary folder; removed when the run ends |
| Daemon runs, their logs and history | `~/.truss/runs/` |
| Secrets | `~/.truss/secrets.json`, readable only by you |
| The daemon's settings | `~/.truss/config.yaml` |

## Local and daemon runs

`truss run` and the daemon use the same engine, so a flow behaves the same way in both. The
differences: the daemon starts runs on its own (schedules, hooks), keeps their history, and runs
them with the environment it started with, which can be shorter than your shell's. See
[the daemon](daemon.md).
