# Feature Specification: TRUSS Desktop

**Spec**: `007-truss-desktop` | **Created**: 2026-11-12 | **Status**: Implemented (first version)

**Input**: "GUI for the TRUSS application: see running jobs, dashboard, control panel of the
job, logs and outputs. Let's call it TRUSS Desktop."

User-facing documentation: [`docs/desktop.md`](../../docs/desktop.md).
Decision: [ADR 0004](../../docs/decisions/0004-desktop-is-an-embedded-web-app.md).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **See what is going on** (P1). One page shows what is running, queued, coming up and what
   happened today.
2. **Control pipelines** (P1). I register a file, run it now, pause its schedule, unregister it.
3. **Understand a run** (P1). I see its steps as a graph with states, each step's duration and
   exit code, and its log, live while it runs. I can cancel it or run it again.

## Requirements

- **FR-001** `truss desktop` prints the address and opens it in the default browser;
  `--no-open` only prints it. Without a daemon it fails with the start hint.
- **FR-002** The application is served by the daemon and uses only the public v1 API.
- **FR-003** The access token is passed in the URL fragment and sent as a header. Without a
  valid token the page explains how to open it.
- **FR-004** Dashboard: counters (registered, running, queued, and today's succeeded, failed,
  skipped), active runs, the next scheduled time per pipeline, recent runs.
- **FR-005** Pipelines: state (`ready`, `running`, `paused`, `invalid` with the reason), file
  path, schedules with the next time, last run; register form; run now, pause or resume,
  unregister (with confirmation).
- **FR-006** Run: status, trigger, times, reason; step graph by dependency depth; per-step
  status, duration and non-zero exit code; run log that grows while the run is active;
  selecting a step filters the log to it; cancel while active, run again afterwards.
- **FR-007** Views refresh without user action. A form being typed in is not reset by a refresh.
- **FR-008** Text from pipelines, logs and errors is always inserted as text, never as markup.
  The daemon sends a Content-Security-Policy that allows only its own origin.
- **FR-009** Light and dark appearance follow the system setting. Controls are reachable by
  keyboard.

## Out of scope

A native window, tray icon and notifications; editing pipeline files; log search; charts of
history; packaging and auto-start.

## Verification

Exercised by hand and with a headless browser against a live daemon (every page in both
appearances, step filter, cancel, register and unregister). `internal/daemon` tests cover that
the page is served and that the API refuses requests without the token. Automated browser tests
and the file picker were added by [`013-desktop-polish`](../013-desktop-polish/spec.md).
