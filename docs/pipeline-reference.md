# Pipeline file reference

A pipeline is one YAML file, by convention `truss.yaml`. This page lists every field of schema
version `"1"`. Unknown fields are rejected everywhere, so a typo never changes what runs.

Check a file at any time without running it:

```console
$ truss validate
truss.yaml is valid: pipeline nightly, 3 steps, concurrency 2
```

## At a glance

```yaml
version: "1"                  # required

pipeline:
  name: nightly               # required
  concurrency: 2              # steps at once; default 1
  overlap: queue              # queue | skip | parallel; default queue
  workdir: app                # where steps start; default: the run's directory

on:                           # optional; needs the daemon
  schedule:
    - cron: "0 9 * * 1-5"     # weekdays at 09:00
      timezone: Europe/Berlin # default: the machine's local time
      missed: skip            # skip | run; default skip

env:                          # optional; for every step
  STAGE: nightly

jobs:                         # either jobs ...
  build:
    steps:
      - id: compile
        run: make build
        timeout: 10m
  test:
    needs: [build]
    env:
      GOFLAGS: -race
    steps:
      - id: unit
        run: make test
        dir: services/api

# steps:                      # ... or a flat list of steps
#   - id: hello
#     run: echo hello
```

## Top level

| Field | Required | Meaning |
|---|---|---|
| `version` | yes | The string `"1"`. A bare number `1` is rejected. |
| `pipeline` | yes | Name and run settings. |
| `on` | no | What starts the pipeline besides you. |
| `env` | no | Environment variables for every step. |
| `steps` | one of the two | A list of steps that may depend on each other. |
| `jobs` | one of the two | Named groups of steps that depend on each other. |

A file has `steps` or `jobs`, never both.

## `pipeline`

| Field | Default | Meaning |
|---|---|---|
| `name` | required | Shown in logs. Also the default name when you register the file. |
| `concurrency` | `1` | The most steps that run at the same time. A whole number of at least 1. |
| `workdir` | the run's directory | The directory steps start in: relative to where the run starts (your current directory for `truss run`, the file's directory for the daemon), or absolute. A job's or step's `dir` replaces it. The directory must exist, or the step fails with a diagnostic and no exit code. |
| `overlap` | `queue` | What happens when a run is requested while another run of this pipeline is still active. Used by the daemon. |

`overlap` values:

| Value | A second run request ... |
|---|---|
| `queue` | waits and starts when the first run ends. |
| `skip` | is dropped and recorded as skipped. Good for schedules that must not pile up. |
| `parallel` | starts next to the first run. |

## `on.schedule`

A list of cron triggers. They only fire while the [daemon](daemon.md) runs and the pipeline is
registered. `truss run` ignores them.

| Field | Default | Meaning |
|---|---|---|
| `cron` | required | Five fields: minute, hour, day of month, month, day of week. |
| `timezone` | local time | An IANA zone such as `Europe/Berlin` or `UTC`. |
| `missed` | `skip` | What to do with a time that passed while nothing watched the clock. |

Cron syntax:

| Form | Example | Meaning |
|---|---|---|
| value | `30 18 * * *` | every day at 18:30 |
| list | `0 8,17 * * *` | at 08:00 and 17:00 |
| range | `0 9 * * 1-5` | Monday to Friday at 09:00 |
| step | `*/15 * * * *` | every 15 minutes |
| names | `0 9 * * mon-fri`, `0 0 1 jan *` | three-letter weekdays and months |
| shortcut | `@hourly` `@daily` `@weekly` `@monthly` `@yearly` | the usual meanings |

Sunday is `0` or `7`. When both day fields are restricted, a day matches if either one does
(`0 0 13 * 5` is the 13th and every Friday), as in classic cron.

### Missed times

A laptop is not a server: it sleeps, and it is off at night. TRUSS never surprises you with a
late run by default.

| `missed` | The time passed while the machine was off, asleep, or the daemon was stopped |
|---|---|
| `skip` | Nothing runs. The history shows a skipped run with the time it was planned for. |
| `run` | One catch-up run starts as soon as the daemon is back, however many times were missed. |

A schedule counts as on time when it fires within two minutes of its planned time.

## `env`

A mapping of variable names to values, allowed at the top level, on a job, and on a step.
Later levels win: step over job over pipeline over the environment the run started from.
Values are taken as text, so `RETRIES: 3` sets the string `3`.

Every run started by the daemon also sets `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and `TRUSS_TRIGGER`
(`manual`, `schedule` or `catch-up`).

TRUSS has no secret store yet. Do not put secrets in a pipeline file; the run log never prints
variable names or values, but the file itself is plain text.

## `steps`

Each step is one command for the host shell (`/bin/sh -c` on Linux and macOS, `cmd /c` on
Windows). Every step starts a new shell, so `cd` or `export` in one step does not carry over.

| Field | Required | Meaning |
|---|---|---|
| `id` | yes | Unique within the list. |
| `run` | yes | The command line, as you would type it. |
| `needs` | no | Ids of steps that must succeed first. Top-level steps only. |
| `env` | no | Variables for this step. |
| `dir` | no | Working directory, relative to where the run starts: your current directory for `truss run`, the file's directory for the daemon. |
| `timeout` | no | Ends the step when it runs longer: `90s`, `5m`, `1h30m`. |

A step's working directory is the first one set of: its own `dir`, its job's `dir`,
`pipeline.workdir`, the run's starting directory. They replace each other; they are not joined,
so a relative `dir` is always relative to the run's starting directory, not to `workdir`. On
Windows a path with a drive letter must be absolute (`C:\work`, not `C:work`); `\work` means
that folder on the drive the run starts on.

Top-level steps form a graph through `needs`. A step starts when everything it needs has
succeeded and a concurrency slot is free. Without `needs` and with the default concurrency,
steps simply run top to bottom.

## `jobs`

A job is a named list of steps for one purpose: build, test, deploy.

| Field | Required | Meaning |
|---|---|---|
| `steps` | yes | Run in the order written, one after another. |
| `needs` | no | Ids of jobs that must succeed first. |
| `env` | no | Variables for every step of the job. |
| `dir` | no | Working directory for the job's steps; a step's own `dir` replaces it. |

Job ids use letters, digits, `.`, `_` and `-`. In logs and in TRUSS Desktop a step of a job is
named `job/step`, for example `build/compile`.

Jobs that do not need each other run side by side when `pipeline.concurrency` allows it:

```yaml
pipeline:
  name: ci
  concurrency: 3

jobs:
  lint:  { steps: [{ id: vet,  run: go vet ./... }] }
  test:  { steps: [{ id: unit, run: go test ./... }] }
  build:
    needs: [lint, test]
    steps:
      - { id: compile, run: go build ./... }
```

## Data and cache folders

Every run gets two folders that outlive the run. Steps find them in variables:

| Variable | `truss run` | Daemon run |
|---|---|---|
| `TRUSS_DATA_DIR` | `.truss/data` in the current directory | `~/.truss/pipelines/<name>/` |
| `TRUSS_CACHE_DIR` | `.truss/data/cache` | `~/.truss/pipelines/<name>/cache` |

TRUSS creates both before the first step and never empties them. What goes in is up to your
steps: a package cache, a downloaded toolchain. There are no cache keys, no saving or
restoring, and no clean workspace per run; the folders are plain directories. `.truss/` is
where `truss run` also writes `run.log`; add it to `.gitignore`. A `<name>` is the registered
name. Pipeline `env` can override both variables like any other. To clear a cache see
[the daemon page](daemon.md#data-and-cache-folders).

## When something fails

The first failed step fails the run. Steps that are still running are stopped, and nothing new
starts. A step fails when its command exits with a non-zero code, cannot start, is ended by a
signal, or runs past its `timeout`.

## Recipes

**A scheduled job that must never overlap itself**

```yaml
version: "1"
pipeline:
  name: sync-photos
  overlap: skip
on:
  schedule:
    - cron: "*/10 * * * *"
steps:
  - id: sync
    run: rsync -a ~/Pictures/ /Volumes/backup/Pictures/
    timeout: 9m
```

**A morning job that still runs when the laptop opens late**

```yaml
on:
  schedule:
    - cron: "0 9 * * 1-5"
      missed: run
```

**Fan out, then fan in**

```yaml
version: "1"
pipeline:
  name: release
  concurrency: 4
steps:
  - { id: build,   run: make build }
  - { id: unit,    run: make test,  needs: [build] }
  - { id: lint,    run: make lint,  needs: [build] }
  - { id: package, run: make dist,  needs: [unit, lint] }
```
