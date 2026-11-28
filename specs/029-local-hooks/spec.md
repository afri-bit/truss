# Feature Specification: Local hooks

**Spec**: `029-local-hooks` | **Created**: 2026-11-27 | **Status**: Implemented

Extends [`005-jobs-schedules-and-step-settings`](../005-jobs-schedules-and-step-settings/spec.md)
(`on`) and [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md).
Decision: [ADR 0026](../../docs/decisions/0026-hooks-poll-and-run-only-in-the-daemon.md).
Risk class: tier 2 (the flow file schema, runs the user did not start by hand). Scope set by the
maintainer on 2026-11-25 (milestone v0.3.0): local hooks for file and folder changes; webhooks from
outside wait for a daemon reachable beyond localhost. Decided the same day: polling, no new
dependency; no catch-up of changes made while the daemon was off.

## User stories

1. **A folder as an inbox** (P1). I drop photos into `inbox/` and the flow that sorts them runs.
2. **Code changes** (P2). I save a Go file and the test flow runs once, not once per file.
3. **One flow after another** (P1). When `build` succeeds, `deploy` runs; when it fails, `alert`
   runs.

## Requirements

- **FR-001** `on.file` has `paths` (required list), `ignore` (list) and `debounce` (`1s` to `10m`,
  default `2s`). Patterns use `/`, are relative to the project root (the file's folder outside a
  project) or absolute; a pattern without `*`, `?` or `[` names a file or a folder and everything
  under it; `**` spans folders.
- **FR-002** While the daemon runs, for a registered flow that is not paused, a file created,
  changed (time or size) or removed under the paths, not ignored, starts one run (trigger `file`)
  once no further change came for `debounce`. The first look after the daemon starts, the flow is
  registered or resumed, its file loads again after failing, or its `paths` or `ignore` change takes
  a baseline: earlier changes start nothing. A new `debounce` keeps the baseline.
- **FR-003** The run gets `TRUSS_CHANGED_FILES`: the path of a file listing the changed paths, one
  per line, sorted, relative to the project root (absolute for an absolute pattern).
- **FR-004** `.truss/state/` and `.git/` are never watched; links are not followed (a linked base
  is named in the daemon log); a scan looks at no more than 10,000 matching files and 50,000 files
  and folders per flow (the daemon log says when it stops). A name with a control character is
  left out and logged. A pattern may not watch a filesystem root or start with `~`, and has at
  most three `**`; matching costs at most patterns x path segments.
- **FR-004a** A flow has at most one hook-started run queued or running at a time. File changes,
  and on.run matches, seen meanwhile start one run once it ends (decided 2026-11-27 after review:
  dropping an on.run match would leave the latest build undeployed). A file-hook run that cannot
  start is tried again after another `debounce`, five times, then its changes are dropped and
  logged. A pattern that starts with `/` but has no drive on Windows is left out and logged.
- **FR-005** `on.run` is one mapping or a list: `flow` (required) and `status` (`succeeded`,
  `failed`, `any`, or a list; default `succeeded`; `any` is succeeded, failed or cancelled). When a
  daemon-known run of that flow (by its registered name or the name in its file) that started
  ends with a matching status, the flow starts (trigger `run`), unless it is paused. Runs that never
  started (cancelled while queued, skipped, a file that did not load) and runs closed because the
  daemon had stopped start nothing.
- **FR-006** A chain of runs started by `on.run` stops after 10: the eleventh is not started and
  the daemon log says so.
- **FR-007** `truss flows show <flow>` lists the hooks ("Starts when"); `--json` adds `hooks`, a
  list of `{type: "file", paths, ignore, debounce}` and `{type: "run", flow, status}`.
- **FR-008** The list of changed files is written when the run is queued, so a queued run keeps it
  across a daemon restart. Patterns may not go up with `..`. An ignored folder is not walked. Ended
  runs that the daemon was too busy to check are counted in its log.

## Limits

Matching is case-sensitive on every system. Changes are seen by time and size, so a same-size
rewrite within a filesystem's time resolution is missed. Changes made while a hook is edited are
part of the new baseline.

## Not done

Webhooks from outside; catching up changes made while the daemon was off; telling a loop of
`on.run` flows apart before it runs (`truss validate` sees one file at a time).

## Compatibility

Additive: `on.file` and `on.run` were unknown fields before. `TRUSS_CHANGED_FILES` joins the
reserved built-in names. The API's run `trigger` gains the values `file` and `run`.

## Acceptance

`internal/pipeline/pattern_test.go` (patterns and parsing), `internal/daemon/hooks_test.go` (one run
per burst with the changed paths and ignore, the baseline, paused flows, on.run statuses and the
loop limit, scanning skips state and git).
