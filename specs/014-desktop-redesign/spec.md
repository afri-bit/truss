# Feature Specification: Desktop redesign: shell, monitor, settings

**Spec**: `014-desktop-redesign` | **Created**: 2026-11-13 | **Status**: Implemented

Extends [`007-truss-desktop`](../007-truss-desktop/spec.md) and
[`013-desktop-polish`](../013-desktop-polish/spec.md). User docs: [`docs/desktop.md`](../../docs/desktop.md).
Risk class: tier 2 since FR-010 adds a CLI switch (additive; no change to the daemon, its API or the run log).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **See what a run is doing** (P1). I open a run and see which step is running, how far the run
   is, its step graph with the running step highlighted, the live log, and its configuration.
2. **Keep an eye on the machine** (P2). A Monitor page shows the daemon's health and the
   processes it runs.
3. **Review the design without a daemon** (P2). I run `truss desktop --demo` and see every page
   filled with sample data that moves, including failures, a queue and odd names.
4. **Make it mine** (P2). I pick light or dark, density, refresh rate and log options, and
   collapse the sidebar.

## Requirements

- **FR-001** One page with a collapsible sidebar (Dashboard, Pipelines, Runs, Monitor, Settings,
  active runs, daemon status), a header (breadcrumbs, *Jump to…*, connection state with latency,
  theme toggle, settings menu) and a scrollable content area. The collapse button sits at the
  bottom left; `[` toggles it.
- **FR-002** Light and dark themes. *System* follows the operating system; a pinned choice is
  kept in the browser and applied before the first paint.
- **FR-003** Pipeline cards place every element at the same position whatever the content: the
  same rows (name and state, path, schedule, next run, last run, duration, shape, actions) at the
  same heights, and the same buttons in the same order (*Pause schedule* is shown disabled when a
  pipeline has no schedule).
- **FR-004** The run page shows progress (steps done of total), the running steps, start time,
  elapsed time, the supervising process id, the step graph by dependency stage with the running
  step highlighted and its incoming edges animated, a step list, the live log (line numbers,
  find, wrap, follow), and the pipeline's configuration.
- **FR-005** The daemon does not report CPU, memory or per-step processes. Where the page shows
  such figures they are simulated, deterministic per run and step, and every place that shows
  them carries a visible *simulated* label. *Settings → Simulated metrics* turns them off. The
  daemon's own process id, address, version, uptime, run slots, schedules and run results are
  real.
- **FR-006** Monitor shows health checks derived from real data (API latency, run slots,
  scheduler, pipeline files, recent failures, poll availability), and simulated host CPU,
  memory, per-core load and a process table.
- **FR-007** Settings (theme, density, sidebar, motion, refresh interval, simulated metrics, log
  wrap, timestamps, font size) are stored in the browser only. *Forget token* removes the stored
  access token.
- **FR-008** *Jump to…* (`/` or Ctrl/Cmd-K) finds pages, pipelines and recent runs by name.
- **FR-009** Everything in `013-desktop-polish` keeps working: the register form and file picker,
  keyboard order, refresh without losing typed text or focus, empty and error states, and the
  daemon's Content Security Policy (no inline scripts or style attributes).

- **FR-010** `truss desktop --demo` (off by default) needs no daemon. It serves Desktop from the
  binary on a free loopback port with the same security headers as the daemon, prints
  `TRUSS Desktop (demo, sample data, no daemon): <address>#demo`, opens the browser unless
  `--no-open`, and stops on Ctrl-C with exit 0. The page then uses sample data generated in the
  browser: runs progress in real time, and Run now, Cancel, pause, resume, register and unregister
  work on that data. It makes no API request. The header and sidebar say *Demo data*; the
  settings menu offers *Leave demo*. Demo mode lasts for that browser tab only.
- **FR-011** Sample data exists only on the demo server: the daemon does not serve it, so `#demo`
  on a daemon's page leaves the page on real data. Opening a `#token=` link always means real
  mode, also in a tab that was in demo mode, and a token or demo link pasted into an open tab
  takes effect without a manual reload.

## Compatibility

FR-010 adds a switch to `truss desktop`; the command without it behaves as before. The demo
server has no API and answers `/api/` with `404`.

## Not in scope

Real resource metrics from the daemon (a later API addition would replace the simulation), a
packaged desktop window.
