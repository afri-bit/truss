# 0002. Step concurrency, process trees and strict pipeline fields

- Status: Proposed
- Date: 2026-11-01

## Context

Spec 003 runs Steps one at a time and ignores unknown fields. Spec 004 lets ready Steps run
together, which needs a stoppable process tree per Step, a goroutine-safe run log and a clear
failure and interrupt story. It changes process execution and two public contracts (the pipeline
file and the run log). Requirements and answers are in
[`specs/004-step-concurrency/`](../../specs/004-step-concurrency/spec.md); the reasoning behind each
choice is in its [research.md](../../specs/004-step-concurrency/research.md).

## Decision

1. **Schema.** `pipeline.concurrency` is optional in schema v1. It is a decimal whole number of at
   least 1. The default is 1.
2. **Strict fields.** Unknown fields are rejected at every level. This replaces spec 003's ignore rule.
3. **Process trees.** Each Step owns a process tree: a process group on Linux and macOS, a job
   object on Windows. Standard library only; no new module.
4. **Stopping.** After a failure, no new Step starts and every running Step is ended at once and
   forcefully, with no grace period. Leftover processes are ended when a Step's shell exits.
5. **Log.** A stopped Step logs `step <id> stopped because step <other> failed`, then its finish
   line. The finish line shows the Exit Code or a diagnostic. On Windows a stop exits with 21586.
   A death by signal shows a diagnostic and never `-1`. The run log is guarded by a mutex.
6. **Interrupt.** Ctrl-C (all hosts) and SIGTERM (Linux and macOS) end all Steps. `truss` exits 1
   and saves no log.
7. **Non-interactive.** Pipelines must not prompt. On Linux and macOS a terminal prompt hangs
   until interrupt.
8. **Scheduler.** One scheduler goroutine walks the Steps in definition order. At limit 1 it
   reduces exactly to spec 003.

## Consequences

- Good: independent Steps run together; a failure leaves nothing running; the log stays readable.
- Good: one scheduler path for every limit, so sequential and concurrent runs cannot drift.
- Good: no new dependency; the schema and log stay free of Go details.
- Bad: compatibility impact. Unknown fields now fail validation. Sequential runs change in four
  ways: leftovers end with the Step's shell, a signal death shows a diagnostic instead of `-1`,
  Steps run in their own process group on Linux and macOS (see the prompt limitation below), and
  an interrupt ends all Steps. Schema stays version `1` because no release is tagged.
- Bad: a Step that detaches from its tree can survive, and a Step that prompts hangs on Linux and
  macOS. Both are documented limitations.
- Follow-ups:
  - Detect duplicate YAML keys (the last one wins today).
  - Add kill-on-close to the Windows job so a force-killed `truss` ends its Steps.
  - Handle `setsid` escapees on Unix.
  - Reduce the recycled process-group id risk in the cleanup `kill` (accepted for now).
  - Fix or record the two golangci-lint findings on `develop` (`cmd/truss/run.go`, G301 and G304),
    then make the CI lint step blocking.
