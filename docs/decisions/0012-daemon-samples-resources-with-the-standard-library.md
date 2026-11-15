# 0012. The daemon samples resources with the standard library

- Status: Proposed
- Date: 2026-11-13

## Context

TRUSS Desktop showed simulated CPU and memory figures because the daemon reported none. The
maintainer asked for real figures: the machine, the daemon, and each running step.

## Decision

- A new package, `internal/metrics`, samples every 2 seconds and keeps ten minutes of host history
  in memory. The daemon owns its goroutine and serves the latest sample at `GET /api/v1/metrics`
  (additive to v1). The engine reports each step's process id through an optional
  `ProcessEvents` extension of `StepEvents`, so existing event receivers are untouched.
- **Standard library only.** Linux reads `/proc` and sums a step's whole process group (the
  executor already makes each step a group leader). Windows uses kernel32 and ntdll calls, as the
  executor does, and measures the step's shell only. Other systems answer `supported: false`.
- Rejected: `gopsutil` (one uniform API, but a large new dependency tree for one feature);
  running `ps` on macOS (a process start every sample); keeping history in the browser (charts
  start empty on every reload).

## Consequences

- No new module. The figures are honest about their limits (`tree`, `supported`, `reason`).
- macOS shows no figures until someone adds a reader; the page says so. (Added by
  [ADR 0013](0013-macos-resource-figures-through-libsystem.md).)
- Sampling scans `/proc` every 2 seconds on Linux, a few milliseconds of work on a typical machine.
