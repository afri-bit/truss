# Feature Specification: Daemon, registry and scheduling

**Spec**: `006-daemon-registry-and-scheduling` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: "A daemon like Docker has; the CLI communicates with the daemon. Register and
deregister pipelines so the daemon knows where they are. My local environment can have cron
jobs. If a cron job should start at 9 but I turn on my laptop at 11, it should not run.
Concurrency."

User-facing documentation: [`docs/daemon.md`](../../docs/daemon.md) and
[`docs/cli.md`](../../docs/cli.md). Decisions: [ADR 0003](../../docs/decisions/0003-daemon-local-api-and-file-state.md).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **Run a background service** (P1). I start, stop and inspect a daemon from the CLI.
   *Independent test*: `truss daemon start`, `status`, `stop` on a fresh TRUSS home.
2. **Register my pipelines** (P1). I tell the daemon where a file is and can remove it again.
   *Independent test*: `register`, `pipelines`, `unregister`.
3. **Trigger and observe** (P1). I start a run by name, follow its log, list past runs, cancel.
   *Independent test*: `trigger <name> --follow` prints the run log and exits by the result.
4. **Scheduled runs that respect my laptop** (P1). A cron time starts a run when the clock
   reaches it. A time that passed while the machine was off does not start a late run.
   *Independent test*: with a fake clock, last seen 08:00 and now 11:00, a `0 9 * * *` schedule
   yields one `skipped` record and no execution.
5. **Bounded concurrency** (P2). Many triggers do not overload the machine or overlap wrongly.
   *Independent test*: with `--max-runs 1`, a second pipeline's run stays `queued`.

## Requirements

### Daemon lifecycle

- **FR-001** `truss daemon run` serves in the foreground until interrupted. `truss daemon start`
  launches it detached, waits until it answers, and prints its process id and address.
- **FR-002** At most one daemon serves a TRUSS home. A second start reports the running one.
- **FR-003** `truss daemon stop` cancels active runs, waits for them, and removes the endpoint
  file. `truss daemon status` exits 1 when no daemon runs.
- **FR-004** The TRUSS home is `$TRUSS_HOME`, else `.truss` in the user's home directory,
  created with user-only permissions.
- **FR-005** Commands that need the daemon fail with one line that says how to start it.

### Access

- **FR-006** The daemon listens only on a loopback address (default `127.0.0.1:7878`).
- **FR-007** Every API request MUST carry the bearer token written to the user-only endpoint
  file; the token is regenerated at each start. Requests with a non-local `Host` are refused.

### Registry

- **FR-008** `truss register [<path>] [--name <name>]` records an absolute path under a name
  (default: `pipeline.name`). The file MUST load and validate. Re-registering the same name and
  path succeeds; a different path under a taken name is an error.
- **FR-009** `truss unregister <name>` removes the registration and its schedules. It deletes
  neither the file nor past runs.
- **FR-010** The daemon reads a registered file again before every run and within a tick of a
  change on disk. An invalid or missing file is reported as invalid, with the reason, and
  cannot run until fixed.
- **FR-011** `truss pause <name>` stops a pipeline's schedules; `resume` restarts them from now.

### Runs

- **FR-012** A run starts in the directory of its file, with the daemon's environment plus the
  file's `env` plus `TRUSS_RUN_ID`, `TRUSS_PIPELINE`, `TRUSS_TRIGGER`.
- **FR-013** At most `--max-runs` (default 4) runs execute at once; the rest are `queued` and
  start in request order.
- **FR-014** `pipeline.overlap`: `queue` runs one run of a pipeline at a time; `skip` records a
  second request as `skipped`; `parallel` lets runs overlap.
- **FR-015** A run is `queued`, `running`, `succeeded`, `failed`, `cancelled` or `skipped`.
  Each step is `pending`, `running`, `succeeded`, `failed`, `stopped`, `cancelled` or `skipped`.
- **FR-016** Cancelling ends every process of the run. The log closes with
  `summary cancelled steps <n> duration <d>`.
- **FR-017** Each run keeps its record and its run log (the spec 002 format) in the TRUSS home.
  The newest 200 runs are kept. Runs left running by a dead daemon are closed as `failed`.
  *Amended by [spec 010](../010-run-tracking-and-queue/spec.md): runs that were only `queued` are kept and start again.*

### Scheduling

- **FR-018** A schedule fires when the daemon observes its time within the grace period of two
  minutes. Each scheduled time starts at most one run.
- **FR-019** A time observed later than the grace period was missed. With `missed: skip` no run
  starts and a `skipped` run is recorded with the planned time and the reason. With
  `missed: run` exactly one `catch-up` run starts, however many times were missed.
- **FR-020** The daemon saves when it last observed the clock so FR-019 holds across restarts,
  sleep and power-off. A schedule of a newly registered, edited or resumed pipeline counts
  from that moment, never from the past.
- **FR-021** A schedule whose file is invalid at fire time records a `failed` run with the reason.
  The daemon keeps the schedules of the last valid definition while the file is invalid, so the
  fire time is not lost; it forgets them only when the pipeline is paused or unregistered.
- **FR-023** Triggering a pipeline by hand, or reloading an unchanged file, never changes when its
  schedules fire or what counts as missed.
- **FR-024** A registered path must be a regular file of at most 1 MiB; anything else is refused
  (`422`) at registration and fails the run or schedule that reaches it later.
- **FR-025** A run whose log cannot be created ends `failed` with the reason `open the run log`.
- **FR-026** A client trusts an endpoint file only if the process answering at its address reports
  the recorded pid; otherwise the daemon counts as not running and the stale file is replaced.

### API

- **FR-022** The endpoints, statuses and headers listed in `docs/daemon.md` are the v1 contract:
  JSON, `snake_case`, RFC 3339 times, errors as `{"error": "..."}`.

## Out of scope

Starting at login, remote access, multiple users, authentication beyond the local token,
push-style log streaming, retention settings, editing pipelines through the API.

## Verification

`internal/scheduler`, `internal/registry`, `internal/runs` and `internal/daemon` tests (the
last with a fake clock and real HTTP), `cmd/truss/daemon_test.go`, and the daemon cases of
`.github/scripts/e2e-test.sh`.
