# 0006. Retries, continue-on-error and a small condition language

- Status: Proposed
- Date: 2026-11-12

## Context

A failed step failed the whole run and stopped everything. Real pipelines need a flaky step to
be run again, an optional check that must not block, and cleanup that runs after a failure.
This changes execution semantics, the run-log format and the daemon API, all public contracts
(constitution principle V). Spec: `specs/009-retries-and-conditions/spec.md`.

## Decision

1. **Schema `"1"` stays.** The new keys (`retries`, `retry_delay`, `continue-on-error`, `if`)
   are optional and additive. An older TRUSS rejects them as unknown fields, which is a safe
   failure, so no version bump is made. Existing files behave exactly as before.
2. **Attempts are lines, not new steps.** The first attempt writes the same lines as a step
   without retries. A retry adds one `retry n of total in delay` line and then a normal
   `started` line. The API reports one step with the last attempt's result and an `attempts`
   count. No per-attempt history in the API (the log has it).
3. **A tolerated failure is a failure that does not count.** The step is `failed` in every
   report and carries `continued_on_error`. Steps that need it are skipped by default, because
   a step that needs another step expects its work. `failure()` or `always()` opt in.
4. **Fail-fast stays the default.** After a run-failing failure only steps whose condition
   calls `always()` or `failure()` may start. `success()` and plain comparisons do not bypass it.
5. **A hand-written parser, no new dependency.** The language has `||`, `&&`, `!`, `==`, `!=`,
   three status functions, quoted text and `env.NAME`. It is about 300 lines in
   `internal/pipeline`, bounded in length and depth. Status functions look at the steps a step
   `needs`, not at the whole run, so the result does not depend on scheduling order.
6. **Environment values never appear in diagnostics.** A skipped step says its condition is
   false, not what was compared (constitution V, secrets).

## Consequences

- Authors can write cleanup (`if: always()` with `needs`) and optional steps.
- A step with `always()` and no `needs` starts at once and is stopped by a later failure like
  any running step; the docs say to use `needs`.
- A parser that expects one `started` line per step id must handle retries.
- Rejected: a version `"2"` (nothing incompatible), a YAML-level `on_failure` hook (a second
  way to say what `if: failure()` says), `expr`-style libraries (supply-chain cost for a
  grammar this small).
