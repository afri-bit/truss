# 0006. Tracked local runs report to the daemon

- Status: Accepted
- Date: 2026-11-12

## Context

`truss run` executes in the caller's shell and environment and leaves only `.truss/run.log`
(ADR 0003, decision 8). It never appears in `truss runs` or TRUSS Desktop. Handing the run to the
daemon would fix that and break the reason `run` exists: the run would use the daemon's
environment, not the caller's. The maintainer wants the run to stay where it is and to *report*.

## Decision

1. **Opt in with `truss run --track`.** Without the flag nothing changes and no daemon is needed.
2. **The run stays in the caller's process.** The CLI reports what happens; the daemon records it.
   A tracked run is a `local` run (`trigger: "local"`), never started, queued or restarted by the
   daemon. It takes no slot of `--max-runs` and is invisible to `overlap: skip`, because it is not
   the daemon's work.
3. **Additive endpoints in API v1**, all behind the token:

   | Endpoint | Does |
   |---|---|
   | `POST /runs` | Opens a run in state `running` from `{pipeline, steps:[{id, needs}]}`; `201` with the run. |
   | `POST /runs/{id}/steps` | Reports one step: `{id, status, exit_code?, error?}`. The daemon stamps the times. |
   | `POST /runs/{id}/log?offset=N` | Appends raw run-log text at the current end of the log; `204` with `X-Truss-Log-Offset` and `X-Truss-Run-Status`. |
   | `POST /runs/{id}/finish` | Closes the run: `{status: succeeded\|failed\|cancelled, reason?}`. |

   Existing endpoints and fields keep their meaning. The new trigger value `local` is the only
   change a client of the existing endpoints can see; clients must already tolerate triggers they
   do not know.
4. **Only `local` runs that are `running` accept reports.** A report to a run the daemon executes,
   or to a finished run, is `409`. A client can never write into the log or the steps of a run the
   daemon is responsible for.
5. **Limits.** A log append is at most 256 KiB, JSON bodies at most 1 MiB, a run at most 1000
   steps, names at most 128 bytes without control characters, reasons at most 2000 bytes. Over the
   limit is `413` or `400`. A log append must start at the end of the log; a byte-identical repeat
   of what the log already holds is accepted (`204`) so a lost answer can be retried (the CLI itself
   does not retry: one failed report ends tracking), anything else is `409`.
6. **No daemon: `--track` fails.** The user asked for tracking, so `truss run --track` exits `1`
   with the usual "start it with" hint *before* running anything. It never silently degrades.
   `--track` is a statement about where the run is recorded; a pipeline that did not get
   recorded is a surprise, and the fix is one command. Without the flag, `truss run` works as
   before.
7. **The observer never changes the outcome.** If the daemon becomes unreachable or closes the
   run while it executes, tracking stops with one warning on stderr and the run carries on. Its
   output, log file and exit code are those of the pipeline.
8. **Daemon-side Cancel on a local run is cooperative.** The daemon cannot, and should not, kill a
   process in someone's shell. `POST /runs/{id}/cancel` closes the record as `cancelled` at once
   (`cancelled from the daemon; the local run is asked to stop`). The CLI reports about four
   times a second, sees the status in every answer, and ends the run's processes the way an
   interrupt does; it exits `1`, writes no run log (spec 001) and says
   `cancelled from the daemon`. A run that cannot reach the daemon cannot be cancelled by it.
9. **A daemon that stops forgets nothing but cannot wait.** A graceful stop or a crash closes a
   `running` local run as `failed` (`the daemon stopped while this run was active`), exactly like
   a crashed daemon run. The local process continues and, per decision 7, stops reporting.

## Consequences

- Desktop and `truss runs` show local runs with a `local` badge; the log and step graph work
  unchanged because the log is the same run-log text.
- The CLI sends a request a few times per second while a tracked run lives. Fine on loopback.
- A local run whose CLI is killed (SIGKILL, power loss) stays `running` until someone cancels it in
  Desktop or the daemon restarts. There is no lease; add one if this hurts.
- Any process holding the token can open local runs and fill the history, up to the 200-run limit,
  and can write the log of a *local* run. The token already grants running commands, so this adds
  no authority.
- Report times are the daemon's clock, not the caller's. Both are one machine.

## Alternatives considered

- **Hand the run to the daemon.** Rejected by ADR 0003: wrong environment.
- **Track by default.** Rejected: `run` must work without a daemon and without surprises.
- **Degrade quietly when no daemon runs.** Rejected, decision 6.
- **Send the whole record at the end.** Simpler, but a running run would not be visible, and
  cancel could not work.
- **Server-sent events or a long-lived connection.** More moving parts than a poll that also
  carries the cancel signal.
- **Daemon kills the local process.** It does not own that process and may not even share its
  user namespace later.
