# 0026. Hooks poll, and run only in the daemon

- Status: Proposed
- Date: 2026-11-27

## Context

The maintainer asked for local hooks in v0.3.0: start a flow when files or folders change, and
later webhooks once the daemon is reachable from outside. Watching files can use OS notifications
(a new dependency, different on every system, a watch per folder on Linux) or polling. Hooks start
processes nobody started by hand, so they are an execution-safety concern (constitution V).

## Decision

1. Poll every tick (one second) for modification time and size, as the daemon already does for
   flow files. No new dependency; the same behaviour on every system; bounded at 10,000 files per
   flow.
2. Hooks live in the daemon only: a registered, unpaused flow; `truss run` never watches.
3. No catch-up: the first look takes a baseline. A daemon that was off does not replay changes.
4. A burst of changes is one run, after a quiet `debounce`; `overlap` handles changes during a run.
5. `on.run` follows any daemon-known run; chains stop at 10 to end loops at run time, since
   validation sees one file at a time.
6. Hooks are triggers of their own (`file`, `run`) beside `manual` and `schedule`, so webhooks can
   be added the same way later.

## Consequences

- Up to a second of delay, and a scan per tick for each watching flow; large trees need narrow
  patterns.
- A flow that writes where it watches starts itself; the docs say to `ignore` such paths.
