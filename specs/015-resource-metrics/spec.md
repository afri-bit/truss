# Feature Specification: Resource figures from the daemon

**Spec**: `015-resource-metrics` | **Created**: 2026-11-13 | **Status**: Implemented

Replaces the simulated figures of [`014-desktop-redesign`](../014-desktop-redesign/spec.md)
(FR-005) with real ones. User docs: [`docs/daemon.md`](../../docs/daemon.md#http-api),
[`docs/desktop.md`](../../docs/desktop.md). Decision:
[ADR 0012](../../docs/decisions/0012-daemon-samples-resources-with-the-standard-library.md).
Risk class: tier 2 (daemon API addition, reads other processes' state). Scope decided by the
maintainer on 2026-11-13: resource metrics API; standard library only, Linux and Windows; history
kept by the daemon.

## User stories

1. **See what a run costs** (P1). On a running run I see its process ids and how much CPU and
   memory its steps use, measured by the daemon.
2. **Watch the machine** (P1). Monitor shows the machine's CPU, per-core load, memory and load
   average, with the last minutes as a chart even right after I open the page.
3. **Know when figures are missing** (P2). On an OS where the daemon does not measure, the page
   says so instead of showing invented numbers.

## Requirements

- **FR-001** A run's step carries `pid`: the operating-system process id of its current attempt,
  or of its last attempt once the step waits to retry or has ended. A new attempt clears it until
  its process has started; an attempt whose command did not start leaves none. Steps of tracked
  local runs have none.
- **FR-002** `GET /api/v1/metrics` (token required, like every call) answers
  `{supported, os, reason, interval_seconds, sampled_at, host, daemon, steps, history}`:
  - `host`: `cpu_percent`, `cores[]` (busy percent per logical core), `memory_total_bytes`,
    `memory_used_bytes`, `load[]` (1, 5, 15 minutes; Linux only).
  - `daemon` and each of `steps[]` (`run_id`, `step_id` plus the process fields): `pid`,
    `cpu_percent`, `rss_bytes`, `threads`, `processes`, `tree`.
  - `history[]`: `{at, cpu_percent, memory_used_bytes}`, oldest first, at most ten minutes.
  - CPU percentages are shares of the whole machine: 100 means every core is busy.
- **FR-003** The daemon samples every 2 seconds while it serves, from the start, and keeps ten
  minutes of host history in memory only. It starts no process to do so and writes nothing. Only
  attempts running now are measured: a step waiting to retry is not, so a reused process id is
  never credited to it.
- **FR-004** Linux: figures come from `/proc`. A step is measured with its whole process group
  (`tree: true`), which holds everything it started unless a process left the group. CPU time of
  children that already exited is kept, through their parent's reaped time.
- **FR-005** Windows: figures come from system calls. A step is measured by its shell process only
  (`tree: false`), without `threads`, and the host has no `load`.
- **FR-006** Other systems (macOS today): `supported: false`, `reason` says why, `daemon.pid` is
  set, `steps` and `history` are empty, no `host`. The page shows the reason where figures would be.
- **FR-007** Desktop shows only the daemon's figures; nothing is simulated outside demo mode.
  Spec 014's *Simulated metrics* setting is removed. In demo mode, demo.js answers `/metrics` with
  sample figures, and resource panels carry a *sample* label.
- **FR-008** A run's chart on its page is built from the samples the page has seen while the run
  was running; the host charts use the daemon's history.

## Compatibility

Additive to API v1: a new endpoint and a new optional `pid` field on steps. Clients that ignore
unknown fields are unaffected. The run log and the pipeline file are unchanged.

## Not in scope

macOS figures, per-step history kept by the daemon, disk and network figures, limits or alerts.
