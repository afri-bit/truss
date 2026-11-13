# Feature Specification: Retries, continue-on-error and conditions

**Spec**: `009-retries-and-conditions` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: "retries, `continue-on-error` and `if:` conditions, the smallest useful version."

This spec extends pipeline schema `"1"` with three optional step fields and one optional job
field. It changes the meaning of "a failure fails the whole run" only for definitions that use
them. The user-facing reference is [`docs/pipeline-reference.md`](../../docs/pipeline-reference.md).
Decision record: [ADR 0009](../../docs/decisions/0009-retries-conditions-and-continue-on-error.md).
Tier 2 (pipeline schema, run-log format, daemon API, execution semantics).

## User stories

1. **Retry a flaky step** (P1). As a developer I add `retries: 2` to a step that talks to the
   network so a blip does not fail the run. *Independent test*: a step that fails once and then
   succeeds, with `retries: 1`, gives a succeeded run, and the log shows two attempts.
2. **Tolerate a failure** (P1). As a developer I mark an optional check `continue-on-error: true`
   so its failure is visible but does not stop the run. *Independent test*: a failing tolerated
   step and an independent step: the run succeeds and both are in the log.
3. **Run by condition** (P1). As a developer I clean up after a failure with `if: always()`, and
   deploy only when `env.STAGE == 'prod'`. *Independent test*: a step with a false condition is
   logged and shown as skipped and does not start.

## Requirements

### Retries

- **FR-001** A step may set `retries`: a whole number from 0 to 100 (default 0), the number of
  extra attempts after a failed one. A step runs at most `retries + 1` times.
- **FR-002** A step may set `retry_delay`, a duration in the format of `timeout` (`90s`, `5m`,
  `1h30m`), the wait before each retry. It defaults to no wait. `retry_delay` without `retries`
  of at least 1 is a validation error.
- **FR-003** Any failed attempt is retried: a non-zero exit code, a command that cannot start,
  an end by signal, or a `timeout` (which applies to each attempt on its own).
- **FR-004** A step is not retried when the run was interrupted, or when TRUSS stopped it
  because another step failed the run. A step that may start after a failure (FR-016) and then
  fails on its own is retried like any other. A step that is waiting for a retry when another step
  fails the run is stopped and not run again.
- **FR-005** The result of a step is its last attempt: its exit code, error, output and times.
  The step holds its concurrency slot while it waits to retry.
- **FR-006** A retried step that succeeds does not fail the run. A step whose last attempt
  fails is a failure as in FR-009 or FR-010.

### continue-on-error

- **FR-007** A step may set `continue-on-error: true` (YAML boolean; default `false`). When the
  step fails, after any retries, the failure is logged and shown as failed, and:
  the run does **not** fail, running steps are **not** stopped, and no step is blocked from
  starting by it, except as FR-008 says.
- **FR-008** A step that needs a failed `continue-on-error` step is skipped (the default
  condition, FR-014) unless its `if` says otherwise, for example `failure()` or `always()`.
- **FR-009** A step that failed with `continue-on-error` is still a failed step: it has status
  `failed` and the exit code or diagnostic of its last attempt. `truss run` exits 0 and the run
  status is `succeeded` when no other step failed the run.
- **FR-010** A failed step without `continue-on-error` fails the run as before: steps still
  running are stopped and nothing new starts, except steps allowed by FR-016.

### Conditions

- **FR-011** A step and a job may set `if`, a string holding an expression (grammar below). An
  invalid expression is a validation error, reported with the step or job, before anything runs.
- **FR-012** Grammar: `||`, `&&`, `!`, parentheses, comparisons `==` and `!=`, the calls
  `always()`, `success()` and `failure()`, text literals in single or double quotes (no
  escapes), and `env.NAME`. An expression is at most 500 characters. Nothing else is accepted:
  no other functions, no arithmetic, no `<` or `>`, no step outputs.
- **FR-013** `env.NAME` is the value of NAME in the environment the step would start with (the
  run's environment plus `env` at pipeline, job and step level, later levels winning), or the
  empty string when it is unset. Names are case-insensitive on Windows. The value is never
  written to the run log, an error message or the API: a skipped step says only that its
  condition is false.
- **FR-014** Status functions look only at the steps the step `needs`. `success()` is true when
  every one of them succeeded, `failure()` when at least one failed (a step that was stopped,
  timed out, or failed under `continue-on-error` counts as failed), `always()` is always true.
  A step without `needs` has `success()` true and `failure()` false. A step that was skipped
  is neither succeeded nor failed.
- **FR-015** A step without `if` has the default condition `success()`. An expression with no
  status function behaves as `success() && (expression)`.
- **FR-016** After a failure that fails the run, a step starts only when its own condition or its
  job's calls `always()` or `failure()`. A step with another condition, with `success()`, or
  with none does not start, as before. Such a step started after the failure is not stopped by
  that failure, and is stopped by an interrupt like any step. A step with no `needs` and
  `always()` starts at once, so a failure that comes later stops it like any running step:
  put the steps it follows in `needs`.
- **FR-017** A job's `if` is evaluated once, when its first step is ready, against the jobs it
  `needs` (the steps they end with). When false every step of the job is skipped, whatever the
  step's own `if`. A job whose first step is skipped because a step it needs did not succeed, or
  because the run had failed, is skipped as a whole in the same way; the later steps are reported
  as `job "<job>" did not start` (silently in the run log after a failure, as FR-018 says).
  A first step skipped by its own `if` does not skip the job. When true the job's condition replaces the default for its first step, and
  the later steps of the job follow their own condition or the default (the step before them
  succeeded).
- **FR-018** A step skipped by a condition, or because a step it needs did not succeed, is
  reported as skipped (below) and counts as skipped for the steps that need it. A step that
  never started only because an earlier failure ended the run is not reported as skipped in the
  run log, which keeps the log of existing definitions unchanged; the API still lists it as
  skipped.

### Run log (contract)

- **FR-019** Every attempt writes its own `started` and `finished` lines and output lines
  exactly as a step without retries does (spec 002). The first attempt of a step with retries
  looks like a step without them.
- **FR-020** New lines, each with the usual timestamp prefix:
  - `step <id> retry <n> of <total> in <delay>` after the `finished` line of an attempt that
    will be retried; `<n>` is the next attempt, `<total>` is `retries + 1`, `<delay>` is in the
    duration format of the log (`1.500s`). The next attempt starts with its `started` line.
  - `step <id> failed but the run continues (continue-on-error)` after the `finished` line of a
    tolerated failure.
  - `step <id> skipped because <reason>` where reason is `step <other> did not succeed`,
    `its if condition is false` or `the if condition of job "<job>" is false`.
  - A step stopped while it waits to retry gets `step <id> stopped because step <other> failed`
    and no further `finished` line.
- **FR-021** The summary line and the `error` line keep their format. `steps <n>` counts steps
  that started, not attempts and not skipped steps. The `error` line names the first step that
  failed the run.

### Daemon API (contract)

- **FR-022** A step in a run (`/api/v1/runs/...`) gains three optional fields, omitted when
  empty: `attempts` (the number of the current or last attempt, present only once a step was
  run again), `continued_on_error` (true for a tolerated failure) and `reason` (why a step was
  skipped). `status` values are unchanged; a skipped step has status `skipped`. The exit code,
  error and times describe the last attempt (`started_at` is when it began); while a step waits
  for a retry, or when it is stopped or cancelled then, it keeps the exit code and error of the
  last attempt that ran, and `attempts` counts attempts that started. A step that waits to retry has status `running`.
- **FR-023** A definition without the new fields produces the same API and log as before.

## Compatibility

Pipeline schema `"1"` is extended, not changed: every new key is optional, and a definition that
does not use them runs exactly as before. The reverse does not hold. Before this release the
keys `retries`, `retry_delay`, `continue-on-error` and `if` were rejected as unknown fields, so a
file that uses them fails to validate on an older TRUSS with `unknown field`, which is the safe
failure. No version bump is needed (ADR 0009).

Run log: three line shapes are new (FR-020) and appear only when the new fields are used.
Parsers that read `started`, `finished`, `summary` and `error` lines keep working; a parser that
assumed one `started` line per step id must expect more. API: three optional fields, additive.
Execution semantics: "the first failure fails the run and stops everything" still holds for
definitions without `continue-on-error` and without conditions. It is relaxed only where the
author asked for it.

## Out of scope

Step outputs and `needs.<id>.result` in expressions, `matrix`, comparison operators other than
`==` and `!=`, expression use in `run` or `env`, exponential back-off, retrying a whole job, a
job-level `continue-on-error`, and a pipeline-level default for `retries`.

## Verification

Tests: `internal/pipeline` (`condition_test.go`, `retry_condition_test.go`: grammar, evaluation,
validation messages), `internal/engine` (`retry_condition_test.go`: attempts, delay, abort,
continue-on-error, conditions, jobs, fail-fast), `internal/runlog`, `internal/runs` and
`internal/daemon` (step states, API fields, log), and four cases in `.github/scripts/e2e-test.sh`
(`retries`, `continue-on-error`, `always()`, `invalid if`) that use commands valid for both
`sh` and `cmd`.
