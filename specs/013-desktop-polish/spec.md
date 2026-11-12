# Feature Specification: Desktop polish: UI tests and a file picker

**Spec**: `013-desktop-polish` | **Created**: 2026-11-12 | **Status**: Implemented

Extends [`007-truss-desktop`](../007-truss-desktop/spec.md). User docs: [`docs/desktop.md`](../../docs/desktop.md),
[`docs/daemon.md`](../../docs/daemon.md). Decision: [ADR 0006](../../docs/decisions/0006-desktop-file-picker-lists-names-only.md).
Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **Register without typing a path** (P1). In the register form I press *Browse…*, walk through
   folders from my home directory, and pick a `.yaml` or `.yml` file.
2. **Trust the interface** (P1). Desktop is covered by automated browser tests that run in CI.
3. **Smoother use** (P2). Empty and error states say what to do; the keyboard works through the
   register form and the lists.

## Requirements

- **FR-001** `GET /api/v1/fs?path=&hidden=` lists one directory as `{path, parent, roots,
  entries[{name, kind, link}], truncated}`. It needs the token like every API call.
- **FR-002** It returns names only, never content, size or time. Entries are directories and
  regular files ending in `.yaml` or `.yml` (any case). Dot-names are left out unless
  `hidden=true`. Directories come first, then files, each sorted by name ignoring case.
- **FR-003** An empty `path` means the user's home directory.
- **FR-004** A path that is not absolute, contains a `..` element (either separator) or a NUL
  byte is `400`. On Windows only `X:\...` or `X:/...` is accepted: drive-relative, UNC,
  `\\?\`, `\\.\` and `:` after the drive are `400`. A missing path is `404`, an unreadable one
  `403`, a file `400`.
- **FR-005** The listed directory is resolved through symbolic links and `path` is the real
  path; `parent` is its real parent, empty at a root. A link is listed with `link: true` and the
  kind of its target. Broken links and links to other things are not listed.
- **FR-006** At most 5000 entries; `truncated` is true when there were more.
- **FR-007** Register form: *Browse…* opens a dialog that starts at the folder of the typed path
  (or home), with Up, Home, drive buttons where there are several roots, and a *Show hidden*
  switch. Picking a file fills the path field and returns focus to it. Escape closes the dialog.
  Refreshes do not close the dialog or reset what was typed.
- **FR-008** Keyboard order in the register form: path, Browse, name, Register. Run rows, step
  rows and step graph nodes are reachable and activate with Enter or Space. A refresh keeps
  keyboard focus where it was.
- **FR-009** States: without a daemon the page says "Daemon unreachable" and how to start it,
  and recovers by itself; an unexpected API error offers *Try again*; an empty or filtered run
  log says so instead of showing a blank box.
- **FR-010** Automated browser tests (`tests/ui/`) cover the dashboard counters, register through
  the picker, run now, pause and resume, unregister, the run page (graph, live log, step
  filter, cancel, run again), the unauthorized state, the unreachable state, empty states, and
  both color schemes loading without console errors. They run in CI on Linux.

## Out of scope

Reading or previewing file content; creating folders; editing pipeline files; UI tests on macOS
and Windows (the suite is portable Node; only Linux runs it in CI for now).

## Compatibility

Additive to API v1: one new endpoint, no change to existing ones.

## Verification

`internal/daemon/browse_test.go`: listing, filtering, hidden names, home default, traversal and
malformed paths, symlinks, roots, token. Windows path rules are tested on every OS through
`checkBrowsePath`; the Windows drive-root listing itself runs only in the Windows CI job.
`tests/ui/*.test.js`: the browser tests above.
