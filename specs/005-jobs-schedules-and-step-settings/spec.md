# Feature Specification: Jobs, schedules and step settings

**Spec**: `005-jobs-schedules-and-step-settings` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: "Various possibilities of the configuration file, from cron job to multiple or
dependent jobs. GitHub Actions grade."

This spec extends pipeline schema `"1"`. The user-facing reference is
[`docs/pipeline-reference.md`](../../docs/pipeline-reference.md); this file records the
requirements behind it. Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **Group work into jobs** (P1). As a developer I describe build, test and release as named
   jobs so that the file reads like my process and independent jobs can run side by side.
   *Independent test*: a file with jobs `build` and `ship` (`ship` needs `build`) runs
   `build/…` steps in order, then `ship/…`.
2. **Schedule a pipeline** (P1). As a developer I add a cron line to the file so the daemon
   runs it for me. *Independent test*: `truss validate` accepts the file and prints the schedule.
3. **Tune a step** (P2). As a developer I give a step its own environment, working directory
   and time limit. *Independent test*: a step with `timeout: 1s` that sleeps fails the run with
   "timed out after 1s".

## Requirements

- **FR-001** A definition MUST have exactly one of `steps` and `jobs`.
- **FR-002** `jobs` is a mapping of job id to job. A job has `steps` (at least one) and may have
  `needs`, `env` and `dir`. Job ids use letters, digits, `.`, `_`, `-`.
- **FR-003** Steps of a job run in the order written, one at a time. A job starts when every job
  it needs has succeeded. Job steps MUST NOT declare `needs`.
- **FR-004** A step of a job is identified as `job/step` in the run log, results and API.
- **FR-005** Job `needs` is validated like step `needs`: unknown, self, repeated and cyclic
  references are errors.
- **FR-006** `on.schedule` is a non-empty list. Each entry has `cron` (five fields or an
  `@`-shortcut) and may have `timezone` (IANA name) and `missed` (`skip` default, or `run`).
  Invalid expressions, expressions that can never match, and unknown zones are errors.
- **FR-007** `truss run` MUST ignore `on`. Schedules are acted on only by the daemon (spec 006).
- **FR-008** `env` is allowed at pipeline, job and step level. Names match
  `[A-Za-z_][A-Za-z0-9_]*`; values are single scalars taken as text. Precedence: step, job,
  pipeline, then the environment the run started from.
- **FR-009** `dir` on a step or job sets the working directory, relative to the run's starting
  directory. A step's `dir` replaces its job's. A directory that does not exist fails the step
  with a diagnostic and no exit code.
- **FR-010** `timeout` on a step is whole hours, minutes and seconds (`90s`, `5m`, `1h30m`).
  A step that exceeds it is ended with its whole process tree and fails with the diagnostic
  `timed out after <duration>` and no exit code.
- **FR-011** `pipeline.overlap` is `queue` (default), `skip` or `parallel`.
- **FR-012** `pipeline.concurrency` limits running steps across all jobs.
- **FR-013** Unknown fields remain errors at every new level. The run log and every error
  message MUST NOT print environment variable names or values.
- **FR-014** `truss validate [<path>]` checks a definition without running it, prints one
  summary line, each schedule and each step with its needs, and exits 0; an invalid file prints
  the same `error:` line `truss run` would and exits 1.
  *Changed by [spec 016](../016-project-layout/spec.md): `truss validate` with no argument checks `truss.yaml` and every flow. `validate <name|path>` checks one pipeline (spec 016 FR-011).*

## Compatibility

Additive. Every definition valid under specs 001 to 004 keeps its meaning. No version bump.

## Out of scope

Matrix builds, conditions (`if`), retries, `continue-on-error`, artifacts and caches, a secret
store, non-cron triggers (file changes, webhooks), job-level timeouts.

## Verification

`internal/pipeline/schema_test.go`, `internal/cron/cron_test.go`,
`internal/engine/stepconfig_test.go`, `cmd/truss/daemon_test.go` (validate), and the `jobs` and
`validate` cases of `.github/scripts/e2e-test.sh`.
