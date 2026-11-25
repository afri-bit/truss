# Feature Specification: Workspace and cache folders

**Spec**: `008-workspace-and-cache` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: "A pipeline-level working directory, and a per-pipeline folder that survives runs, so
a step can keep a package cache or a toolchain between runs."

Tier 2 (pipeline schema, process environment, a CLI command). The user-facing text is in
[`docs/pipeline-reference.md`](../../docs/pipeline-reference.md) and
[`docs/daemon.md`](../../docs/daemon.md). Written alongside the implementation.

## User stories

1. **Run everything in a subfolder** (P1). As a developer with a monorepo I set one `workdir`
   instead of repeating `dir` on every step. *Independent test*: a file with `workdir: app`
   and two steps runs both in `app`; a step with its own `dir` runs there instead.
2. **Keep files between runs** (P1). As a developer I use `$TRUSS_CACHE_DIR` for a download
   cache so the nightly run does not fetch everything again. *Independent test*: two daemon
   runs of one pipeline see the same `TRUSS_CACHE_DIR` path, and a file written by the first
   is there in the second.
3. **Clear it** (P2). As a developer I empty a pipeline's cache when it is corrupt.
   *Independent test*: `truss cache clear <name>` removes `cache/` and nothing else.

## Requirements

- **FR-001** `pipeline.workdir` is an optional non-empty string. Empty, non-string and
  non-scalar values are errors (`flow workdir must be a non-empty string`).
- **FR-002** A step's working directory is the first one set of: the step's `dir`, its job's
  `dir`, `pipeline.workdir`, the directory the run starts in. The settings replace each other;
  they are never joined. A relative value is always relative to the run's starting directory
  (the file's folder for the daemon, the current directory for `truss run`), including a
  relative `dir` next to a `workdir`.
- **FR-003** An absolute path is used as written. On Windows a path with a drive letter but
  no root (`C:work`) is refused with a diagnostic, because its meaning depends on a per-drive
  current directory; a path with a root but no drive (`\work`, `/work`) is taken on the drive
  of the run's starting directory.
- **FR-004** A working directory that does not exist, or is refused by FR-003, fails the step
  with a diagnostic and no exit code, exactly as a missing step `dir` does (spec 005 FR-009).
  The failure is reported when the step starts, not at validation.
- **FR-005** For every daemon run the daemon creates `<home>/pipelines/<name>/` and its `cache/`
  folder (private to the user) before the run is queued, where `<name>` is the registered name,
  and every step gets `TRUSS_DATA_DIR` (the first folder) and `TRUSS_CACHE_DIR` (the second).
  Both are absolute paths. They sit with `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and `TRUSS_TRIGGER`
  below the pipeline's `env`, so a file may override them like any variable.
- **FR-006** TRUSS never deletes the folders of FR-005: not at the end of a run, not on
  unregister. Registering the name again finds the old contents.
- **FR-007** `truss run` creates `.truss/data/` and `.truss/data/cache/` in the current
  directory (the folder that already holds `run.log`) and sets the same two variables to
  their absolute paths. If `.truss`, `.truss/data` or `.truss/data/cache` is a symbolic link, the
  run fails before any step starts (specs/002-local-run-log FR-018). **Decision:** a separate `data` folder keeps step files apart from
  TRUSS's own `run.log`, and the rule "cache is `$TRUSS_DATA_DIR/cache`" holds in both modes.
  The folders are per project and per current directory, not per pipeline name. If they cannot
  be created `truss run` exits 1 with `error: prepare data folder: ...` before any step runs.
  *Changed by [spec 016](../016-project-layout/spec.md): The folders are now per pipeline (flow), not per project or per current directory: each flow has its own `.truss/state/<flow>/data/` and `.truss/state/<flow>/data/cache/`. The error is now `error: prepare state folder: ...`.*
- **FR-008** `truss cache clear <name>` empties the cache by removing `<home>/pipelines/<name>/cache`
  (the next daemon run creates it again), prints
  `cleared the cache of <name>` (or `no cache for <name>` when there is none) and exits 0. It
  does not contact the daemon, works for unregistered names, and leaves the rest of the data
  folder alone. An invalid name or any other arguments are a usage or `error:` exit 1.
- **FR-009** Unknown fields remain errors. `truss validate` is unchanged.

## Compatibility

**Additive.** A file without `workdir` plans and runs as before (the planner leaves `dir`
untouched), so every definition valid under specs 001 to 005 keeps its meaning. No schema
version bump. Two process-environment changes are visible to steps: `TRUSS_DATA_DIR` and
`TRUSS_CACHE_DIR` are new names; a step that read a variable of the same name from its own
environment now sees TRUSS's value unless the file sets it in `env`. The Windows refusal in
FR-003 applies to `workdir` and also, because they share the resolver, to a step or job `dir`
written as `C:work`, which was never meaningful. `truss run` writes a new `.truss/data`
folder next to `run.log`. No daemon API or run-log change. `.truss` keeps mode 0755 as before; the new `data` folder is 0700. *Changed by [spec 016](../016-project-layout/spec.md): The folder is `.truss/state/<flow>/data`.*

## Known limitations

- The folder name is the registered name. On case-insensitive file systems (Windows, default
  macOS) `Demo` and `demo` share a folder, and Windows reserved device names (`nul`, `con`) and
  a trailing `.` are not rejected at registration. Use distinct, plain names.
- `truss cache clear` is for idle pipelines. It does not check for active runs; on Windows it
  fails on files a running step holds open, on Unix it deletes under a running step, and a
  read-only tree (mode 0555) in the cache makes it fail with a permission error. A run that is
  queued when the cache is cleared gets the folder created again when it starts.

## Out of scope

Restoring or saving cache keys, artifacts, a clean workspace per run, containers, per-run
folders, size limits or eviction, `~` or variable expansion in `workdir`.

## Verification

`internal/pipeline/plan_test.go` (precedence, validation), `internal/engine/stepconfig_test.go`
(precedence by running, absolute, missing, Windows path rules), `internal/daemon/daemon_test.go`
(variables, workdir, data kept on unregister, missing workdir), `cmd/truss/workspace_test.go`
(`truss run` variables and workdir, `cache clear`), and the `workdir` cases of
`.github/scripts/e2e-test.sh`. The Windows cases of `TestStepDir` run only on the Windows CI job.
