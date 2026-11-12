# Feature Specification: Run tracking and a persistent queue

**Spec**: `010-run-tracking-and-queue` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: Two gaps in the daemon (spec 006): queued runs are lost when the daemon stops, and a
`truss run` leaves only `.truss/run.log` so it never shows up in `truss runs` or Desktop.

Builds on [spec 006](../006-daemon-registry-and-scheduling/spec.md) and
[ADR 0003](../../docs/decisions/0003-daemon-local-api-and-file-state.md) (decisions 8 and 9: the
daemon is started separately, and `truss run` is never handed to it).
User-facing documentation: [`docs/daemon.md`](../../docs/daemon.md), [`docs/cli.md`](../../docs/cli.md).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **A restart does not lose my queue** (P1). Runs waiting for a slot are still waiting after the
   daemon stops and starts again, and start in the order they were requested.
   *Independent test*: queue three runs behind a running one, stop the daemon, start it again;
   the queued runs finish in request order.

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

## Out of scope

Resuming a run that was executing, retry policies, and changing the order of the queue.

## Verification

`internal/runs` (restart, order, limits, unrestorable runs, cancel), `internal/daemon` (a restart
over records left by a stopped daemon).
