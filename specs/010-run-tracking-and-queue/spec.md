# Feature Specification: Run tracking and a persistent queue

**Spec**: `010-run-tracking-and-queue` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: Two gaps in the daemon (spec 006): queued runs are lost when the daemon stops, and a
`truss run` leaves only `.truss/run.log` so it never shows up in `truss runs` or Desktop.

Builds on [spec 006](../006-daemon-registry-and-scheduling/spec.md) and
[ADR 0003](../../docs/decisions/0003-daemon-local-api-and-file-state.md) (decisions 8 and 9: the
daemon is started separately, and `truss run` is never handed to it).
User-facing documentation: [`docs/daemon.md`](../../docs/daemon.md), [`docs/cli.md`](../../docs/cli.md).
Decisions: [ADR 0006](../../docs/decisions/0006-tracked-local-runs.md).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **A restart does not lose my queue** (P1). Runs waiting for a slot are still waiting after the
   daemon stops and starts again, and start in the order they were requested.
   *Independent test*: queue three runs behind a running one, stop the daemon, start it again;
   the queued runs finish in request order.
2. **See a run I started in my shell** (P1). `truss run --track` executes exactly like `truss run`,
   in my shell and environment, and the run shows up in `truss runs` and Desktop with its steps
   and log. *Independent test*: with a daemon running, `truss run --track`; `truss runs` lists a
   `local` run and `truss logs <id>` prints what `run` printed.
3. **Stop a tracked run from Desktop** (P2). Cancelling it in Desktop or with `truss cancel` ends
   the local run. *Independent test*: cancel a tracked run that is blocked in a step; the CLI exits
   `1` and the step's process is gone.

## Requirements

### Persistent queue

- **FR-001** A run that is `queued` when the daemon stops gracefully (`daemon stop`, interrupt,
  `POST /shutdown`) stays `queued` in its record. It is not cancelled.
- **FR-002** When the daemon starts it restores every `queued` run, in the order the runs were
  created, and starts them as the limits of spec 006 (FR-013, FR-014) allow. The creation order
  never depends on timestamps being distinct.
- **FR-003** A restored run reads its pipeline file at the moment the daemon starts, as every run
  does (spec 006 FR-010), and keeps its id, trigger and scheduled time. Its log starts when it starts.
- **FR-004** A restored run whose pipeline is no longer registered, or whose file no longer loads,
  is closed as `failed`. Its note says `the daemon stopped while this run was queued and it could
  not be restored:` followed by the cause, and its steps are `skipped`.
- **FR-005** A run that was `running` when the daemon ended is still closed as `failed` with
  `the daemon stopped while this run was active` (spec 006 FR-017). A graceful stop still cancels
  runs that are executing, with `the daemon stopped`. Only queued runs are kept.
- **FR-006** A restored queued run can be cancelled like any other queued run.
- **FR-006a** A restored run shows the steps of the file it will execute, all `pending`, not the
  steps the file had when the run was requested.

### Tracking: the CLI

- **FR-007** `truss run [<path>] --track` runs the pipeline exactly as `truss run` does: in the
  caller's process, shell, working directory and environment, with the same run log on stdout, the
  same `.truss/run.log`, the same exit code. `--track` is the only option of `run`; any other
  argument is the path. (Compatibility: a definition file literally named `--track` must now be
  written `./--track`.)
- **FR-008** `--track` needs a daemon. When none runs, `truss run --track` prints the usual
  `error: ... (start it with: truss daemon start)` line on stderr, exits `1`, writes nothing to
  stdout and runs nothing. A definition that does not load fails as it does without `--track`,
  before the daemon is contacted.
- **FR-009** A tracked run prints one line to stderr, `tracking as run <id> (truss logs <id>)`,
  and nothing extra to stdout.
- **FR-010** Tracking never changes the outcome. If the daemon cannot be reached or closes the run
  while it executes, the CLI prints one `warning:` line on stderr, stops reporting, and the run
  finishes with its own output and exit code.
- **FR-011** A tracked run that is interrupted (Ctrl-C) is closed as `cancelled`, reason
  `interrupted`. A run that fails is closed as `failed` with the same reason a daemon run gets
  (`<pipeline> failed at <step> (exit <n>)`). A run that succeeds is closed as `succeeded`.

### Tracking: the daemon

- **FR-012** A tracked run is a run with trigger `local`. It starts `running` with every step
  `pending`, appears in `GET /runs`, `GET /runs/{id}` and `GET /runs/{id}/log` like any run, and
  counts in `running_runs`, but takes no slot of `--max-runs` and does not count for
  `overlap: skip` or `queue` of its pipeline.
- **FR-013** The endpoints of [ADR 0006](../../docs/decisions/0006-tracked-local-runs.md) open a
  run (`POST /runs`), report a step (`POST /runs/{id}/steps`), append to the log
  (`POST /runs/{id}/log?offset=`) and close it (`POST /runs/{id}/finish`). They need the token like
  every endpoint (`401` otherwise). They are additive: nothing in the v1 contract changed.
- **FR-014** Only a `local` run that is `running` accepts reports. Any other run, or one that
  ended, answers `409` and is not modified. An unknown id is `404`.
- **FR-015** Malformed requests are `400`: no name, a name over 128 bytes or with control
  characters, no steps or more than 1000, empty or repeated step ids, an unknown step, an unknown
  status, a missing or negative offset, a step that needs an undeclared step or itself or forms a
  cycle. Reasons and step errors have control characters replaced by spaces. At most 100 tracked
  runs may be running at once (`409`) and a tracked log holds at most 64 MiB (`413`). Bodies over the limits (256 KiB for a log append, 1 MiB for
  JSON, 2000 bytes for a reason) are `413`.
- **FR-016** A log append must start at the current end of the log (`409` otherwise). A
  byte-identical repeat of an append the log already holds is accepted and changes nothing. A step that has ended cannot
  change (`409`). Steps still open when the run is closed become `skipped` (never started) or
  `cancelled` (running), as for any run.
- **FR-017** The daemon stamps step and run times with its own clock.
- **FR-018** `POST /runs/{id}/cancel` on a running `local` run closes it as `cancelled` at once with
  the reason `cancelled from the daemon; the local run is asked to stop`. The CLI reports about
  four times a second, sees the status in the answer, ends the run's processes as an interrupt
  does, exits `1` and prints `error: cancelled from the daemon`. A cancel that arrives after the
  pipeline finished does not change its exit code (FR-010). Like any interrupted run it saves
  no `.truss/run.log` (spec 001 FR-017).
- **FR-019** When the daemon stops (gracefully or not), a `local` run that is `running` is
  closed as `failed` with `the daemon stopped while this run was active`. Finished local runs are
  history like any other and obey the 200-run limit.
- **FR-020** TRUSS Desktop shows the trigger of a local run as a `local` badge, does not offer
  "Run again" for it (there is nothing registered to run), and offers Cancel while it runs.

## Out of scope

Tracking of `truss validate`, tracking by default or by environment variable, a lease that closes
local runs whose CLI died, resuming a run that was executing, retry policies, changing the order of
the queue, and registering a tracked pipeline.

## Verification

`internal/runs` (restart, order, limits, unrestorable runs, cancel, the tracking methods and
their rejections), `internal/daemon` (a restart over records left by a stopped daemon; the
tracking endpoints with and without the token, bad ids, bodies and sizes, and a write to a daemon
run), `cmd/truss/track_test.go` (`--track` against an in-process daemon: steps, log, failure, the
caller's environment, no daemon, daemon cancel, daemon stopping mid-run), and the `run --track`
cases of `.github/scripts/e2e-test.sh`.
