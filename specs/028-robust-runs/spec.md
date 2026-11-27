# Feature Specification: Robust runs

**Spec**: `028-robust-runs` | **Created**: 2026-11-25 | **Status**: Implemented (part; see Not done)

Extends [`004-step-concurrency`](../004-step-concurrency/spec.md) (stoppable process trees),
[`005-jobs-schedules-and-step-settings`](../005-jobs-schedules-and-step-settings/spec.md)
(`timeout`) and [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md).
Decision: [ADR 0024](../../docs/decisions/0024-steps-are-asked-to-stop-before-they-are-killed.md).
Risk class: tier 2 (process execution, cancellation). Scope set by the maintainer on 2026-11-25
(milestone v0.3.0: robustness).

## Why

A stopped step was killed at once, so it could not remove a lock or a half-written file. On
Windows a step could outlive a crashed daemon. `daemon stop` cancelled runs with no way to let
them finish. A step printing without end could fill the disk with its run log.

## Requirements

- **FR-001** On Linux and macOS, when TRUSS stops a step (timeout, cancel, interrupt, a failure
  elsewhere that ends the run, the daemon stopping), every process of the step's group gets
  SIGTERM. The whole group, not only the shell, has `kill_timeout` to end; what still runs then gets
  SIGKILL. A second Ctrl-C during a local `truss run` kills at once; the first one prints that it is
  stopping the steps.
- **FR-002** `kill_timeout` on a step, and on the flow block as the default of every step. A
  duration greater than zero and at most one minute (service managers kill a stopping daemon after
  about that long); default 10 seconds. `truss daemon stop` waits up to 75 seconds.
- **FR-003** On Windows a stopped step's job is terminated at once, as before. The job object is
  marked kill-on-close, so the steps of a TRUSS process that dies end with it.
- **FR-004** A step TRUSS stopped is failed whatever its exit code, including 0 after handling
  SIGTERM, so `failure()` holds for the steps that need it. One stopped by its `timeout` fails with
  "timed out after ...".
- **FR-005** `truss daemon stop --wait` waits until no run is running or queued, then stops the
  daemon. Ctrl-C ends the wait and leaves the daemon running. `--wait` is valid only with `stop`.
- **FR-006** A daemon run's log keeps at most 64 MiB of step output; at the limit one line says so
  and the rest of the step output is dropped. TRUSS's own lines (steps finishing, the summary) are
  always kept, and the run's record is unaffected.
- **FR-006a** A step's output kept in memory for its result is at most 1 MiB per stream, so a step
  that prints without end cannot exhaust TRUSS's memory; the live log is bounded by FR-006, which
  counts what the log writes, line prefixes included.
- **FR-006b** The launchd agent gives a stopping daemon 75 seconds (`ExitTimeOut`), so steps get
  their `kill_timeout`.
- **FR-007** `truss daemon start` sets `daemon.log` aside as `daemon.log.1` when it is over 10 MiB.

## Already true before this spec

Run records, the registry and the secret store are written to a temporary file and renamed, so a
crash leaves the old or the new file, never half of one. A run that was running when the daemon
stopped is closed as failed at the next start.

## Limits

- A process that left the step's process group on Unix is neither asked nor killed.
- `kill_timeout` has no effect on Windows.
- Stopping a run, and `daemon stop`, can now take up to the longest `kill_timeout` of the steps
  that are running.

## Not done (follow-ups)

- On Linux and macOS a step can outlive a daemon that crashes or is killed with SIGKILL: TRUSS has
  no safe way yet to recognise that process later without risking a process that reused its id.
- A lock so that two daemons can never share one TRUSS home.
- A size cap for the `run.log` of a local `truss run`.
- `daemon.log` of a daemon started at login is not set aside (only `daemon start` does it).
- A third Ctrl-C does nothing more; end TRUSS with your system's tools if a step hangs.

## Compatibility

A stopped step on Linux and macOS now gets SIGTERM first and up to 10 seconds; `truss cancel` and
`daemon stop` can take that long. A script that ignores SIGTERM is killed as before, later. The run
log and exit codes are unchanged; a timed-out step that exits 0 now fails, as a timeout always
should have.

## Acceptance

`internal/engine/stop_test.go` (SIGTERM and cleanup on timeout and cancel, kill after
`kill_timeout`), `internal/runs/caplog_test.go`, `cmd/truss/robust_test.go`.
