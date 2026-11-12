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
| `retries` | no | Extra attempts after a failure, 0 to 100. Default 0. See [Retries](#retries). |
| `retry_delay` | no | Wait before each retry, same format as `timeout`. Needs `retries`. |
| `continue-on-error` | no | `true` lets the run go on when this step fails. Default `false`. |
| `if` | no | A condition; the step runs only when it holds. See [Conditions](#conditions). |

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
| `if` | no | A condition for the whole job, checked once when its first step is ready. |

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

## When something fails

The first failed step fails the run. Steps that are still running are stopped, and nothing new
starts, except the steps that ask for it with `if: always()` or `if: failure()`. A step fails
when its command exits with a non-zero code, cannot start, is ended by a signal, or runs past
its `timeout`. `truss run` then exits 1.

### Retries

```yaml
- id: fetch
  run: curl -fsS https://example.com/data.json -o data.json
  retries: 2          # at most 3 attempts
  retry_delay: 10s    # wait between attempts
```

Any failed attempt is run again, and `timeout` applies to each attempt. The step's result is its
last attempt. In the run log every attempt has its own lines:

```text
step fetch started
step fetch finished duration 0.210s exit 22
step fetch retry 2 of 3 in 10.000s
step fetch started
step fetch finished duration 0.180s exit 0
```

A step is not retried when the run is interrupted or when another step has already failed the
run: it is stopped instead.

### continue-on-error

```yaml
- id: lint
  run: make lint
  continue-on-error: true
```

The failure is logged and shown, but the run does not fail, `truss run` can still exit 0, and
other steps keep going. A step that `needs` a failed `continue-on-error` step is skipped, unless
its `if` says otherwise (for example `if: failure()`).

### Conditions

`if` takes an expression. A step without `if` runs when every step it `needs` succeeded.

| Part | Meaning |
|---|---|
| `success()` | every step this step `needs` succeeded (always true without `needs`) |
| `failure()` | at least one step this step `needs` failed, was stopped, or timed out |
| `always()` | true |
| `env.NAME` | the value of NAME as the step would see it, or empty when unset |
| `'text'` or `"text"` | a literal, no escapes |
| `==` `!=` | compare two values |
| `!` `&&` `\|\|` `( )` | not, and, or, grouping |

An expression with no status function means `success() && (...)`. Quote the whole expression
when it starts with `!` or contains `: `.

```yaml
steps:
  - id: build
    run: make build
  - id: deploy
    needs: [build]
    if: env.STAGE == 'prod'
    run: make deploy
  - id: notify
    needs: [build, deploy]
    if: failure()
    run: ./tell-the-team.sh
  - id: cleanup
    needs: [build, deploy, notify]
    if: always()
    run: make clean
```

On a job, `if` is checked once, when the job's first step is ready. If it is false, every step
of the job is skipped; if it is true it replaces the default check for the first step (the jobs
it needs may have failed, for example a job with `if: always()`).

A skipped step is logged as `step deploy skipped because its if condition is false` (or `because
step build did not succeed`) and shown as skipped in TRUSS Desktop. The log never prints the
values an expression compared. A step with `always()` and no `needs` starts at once, so use
`needs` to run it last. After a failure only steps whose condition calls `always()` or
`failure()` start; `success()` and plain comparisons do not.

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
