# 0024. Steps are asked to stop before they are killed

- Status: Proposed
- Date: 2026-11-25

## Context

Since spec 004 a stopped step's process group (Unix) or job object (Windows) was killed at once.
Steps could not clean up, and on Windows a step could outlive a crashed TRUSS. The maintainer asked
for robustness in v0.3.0. Process execution and cancellation are execution-safety contracts
(constitution V).

## Decision

1. Unix: SIGTERM to the step's process group, then SIGKILL after `kill_timeout` (default 10s,
   settable per step and per flow).
2. Windows: terminate the job at once, as before (no stop request reaches every process), and mark
   the job kill-on-close so its processes end when the TRUSS process that holds it ends.
3. A step that TRUSS stopped counts as stopped whatever its exit code; a timeout always fails it.
4. Orphans after a crashed daemon on Unix are left for later: killing by a remembered process id
   risks hitting an unrelated process that reused it.

## Consequences

- Scripts can clean up; cancel and stop can take up to `kill_timeout`.
- Windows and Unix differ, and the docs say so.
