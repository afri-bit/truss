# Feature Specification: Project layout

**Spec**: `016-project-layout` | **Created**: 2026-11-15 | **Status**: Implemented (Windows path cases are first proved by CI; case-sensitive macOS volumes by review only)

**Input**: A folder can hold one `truss.yaml`, so a project cannot hold several pipelines by
convention, and `.truss/` mixes generated state with anything worth committing. The maintainer
wants a project with many pipelines, like a workflow folder, and a root file that only says
"this is a TRUSS project" (decisions 2026-11-15).

Changes [spec 001](../001-host-pipeline-engine/spec.md) (the default file of bare `run` and the required pipeline name), [spec 002](../002-local-run-log/spec.md) (run log path), [spec 003](../003-step-dependencies/spec.md), [spec 004](../004-step-concurrency/spec.md) and [spec 012](../012-secrets/spec.md) (run log path), [spec 008](../008-workspace-and-cache/spec.md)
(data and cache path), [spec 010](../010-run-tracking-and-queue/spec.md) (run log path) and the default file of `run`, `validate` and `register`
([spec 005](../005-jobs-schedules-and-step-settings/spec.md), [spec 006](../006-daemon-registry-and-scheduling/spec.md)). Plan, tasks, research and quickstart files of older specs are history and still name the old paths.
Decisions: [ADR 0015](../../docs/decisions/0015-project-layout.md). Release scope: `docs/MILESTONE.md`.
User documentation: `docs/pipeline-reference.md`, `docs/cli.md`, `docs/daemon.md`.

**Compatibility note.** Breaking, and allowed: nothing is tagged. The project's `truss.yaml` (in the
working directory) that contains `steps` or `jobs` is no longer a pipeline file; `truss run <path>` still runs any pipeline file.
`.truss/run.log` and `.truss/data` of older builds are left alone and ignored. The pipeline schema
`version` stays `"1"`; `pipeline.name` becomes optional (a relaxation).

## User stories

1. **Many pipelines in one project** (P1). Pipelines are files in `.truss/flows/`; `truss run <name>`
   runs one. *Independent test*: put `build.yaml` and `test.yaml` in `.truss/flows/`; `truss run build`
   and `truss run test` run each.
2. **A root file that marks the project** (P1). `truss.yaml` holds the project's name and nothing
   that runs. *Independent test*: `truss validate` reports the project name and each pipeline.
3. **Generated files stay out of git** (P1). Logs, data and cache live in `.truss/state/`, and a
   `.truss/.gitignore` ignores only that. *Independent test*: run a pipeline in a git repository;
   `git status` shows `.truss/.gitignore` and no `state/` files.
4. **The daemon knows the project** (P2). `truss register build` or `--all` registers flows, and the
   daemon runs their steps in the project root. *Independent test*: register a flow whose step
   prints `pwd`; the run prints the project root.

## Requirements

### Project file

- **FR-001** `truss.yaml` in the working directory marks a TRUSS project. Its fields are
  `version` (required, `"1"`, as in a pipeline file), `project.name` (required, same characters as a registry name) and
  `project.description` (optional). Any other field is an error that names the field and where it
  appears, never its value.
- **FR-002** The project's `truss.yaml` (the one in the working directory) with `steps` or `jobs` is
  rejected with a message that says pipelines live in `.truss/flows/`. `truss run truss.yaml` says
  that `truss.yaml` is project metadata, not a pipeline. The refusal also applies when the same file
  is reached by another spelling, such as `TRUSS.yaml` on a case-insensitive filesystem,
  `../proj/truss.yaml` or a link to it. A file named `truss.yaml` in another folder, given by path,
  is an ordinary pipeline file and runs.
- **FR-003** The project root is the working directory. TRUSS never looks in parent folders. A
  command that needs a project and finds no `truss.yaml` names the folder it looked in.

### Pipelines (flows)

- **FR-004** Pipelines are the regular files `<name>.yaml` or `<name>.yml` directly in
  `.truss/flows/`. `<name>`, the file stem, is the pipeline's flow name.
- **FR-005** `.truss` and `.truss/flows` must be real directories (not links). Entries that are links,
  folders, hidden files or other extensions are ignored; `.YAML` is not an extension TRUSS accepts.
  At most 256 entries are considered; more is an error.
- **FR-006** Two stems that differ only by letter case, or the same stem as `.yaml` and `.yml`, are
  an error that names both files. A stem that is not a valid registry name, ends in `.` or a space,
  or is a Windows device name (`CON`, `PRN`, `AUX`, `NUL`, `COM1`-`COM9`, `LPT1`-`LPT9`) is an error.
- **FR-007** A flow file is opened once, must be a regular file, and is read through that handle up to
  1 MiB. A larger file is an error. A flow that cannot be read or parsed is an error; it is never
  skipped. The same limit applies to a pipeline file given by path.
- **FR-008** `pipeline.name` is optional and defaults to the file stem. When present and different
  from the stem, `validate` prints a note; the run log and the daemon use the declared name.

### Commands

- **FR-009** For `run`, `validate` and `register`, an argument that contains `/` or `\` or ends in
  `.yaml` or `.yml` is a path (relative to the working directory); any other argument is a flow
  name. A name that is not found lists the available flows. TRUSS never tries the other reading.
- **FR-010** `truss run` without an argument is an error that lists the available flows (exit `1`).
  `truss run <name>` and `truss run <path>` run as before, steps in the working directory.
- **FR-011** `truss validate` without an argument checks `truss.yaml` and every flow, and prints each
  result. No `.truss/flows` folder is not an error for `validate`; it says there are no pipelines.
  `validate <name|path>` checks one pipeline.
- **FR-012** `truss register` takes a flow name, a path or `--all`. Without any of them it is an
  error that lists the available flows. `--all` registers every flow, each under its pipeline name
  (`--name` is not allowed with `--all`). After a successful `register --all` (every flow
  registered, none failed) the CLI prints one line, `registered N pipelines (M with schedules)` (since spec 024: `flows`), or
  `registered 1 pipeline (M with schedules)` for one. No summary is printed when any flow failed.
  A name already registered for another file is reported with a hint to use `--name`.

### State

- **FR-013** A local run keeps its generated files in `.truss/state/<flow>/` (`<flow>` is the stem of
  the file run): `run.log` (spec 002, otherwise unchanged), `data/` (`TRUSS_DATA_DIR`) and
  `data/cache/` (`TRUSS_CACHE_DIR`). `state` and everything below it is created with mode `0700`.
- **FR-014** No link is followed to create or write anything under `.truss` (spec 002 FR-018 extends
  to `state`, `<flow>` and `.gitignore`).
- **FR-015** When `.truss/state` is first created, TRUSS also creates `.truss/.gitignore` containing
  `state/`, with a new-file-only open, and never changes an existing one. If it cannot write that
  file it prints a `warning:` and the run goes on. It is the only file TRUSS writes outside `state/`.
- **FR-016** A run of a pipeline given by path writes its state under the working directory in
  `.truss/state/<stem of the file>/`, project or not.

### Daemon

- **FR-017** The daemon runs a registered file's steps in the project root when the file is
  `<root>/.truss/flows/<file>` and `<root>/truss.yaml` exists (checked without following links);
  otherwise in the file's directory, as before. The daemon's rules for registered files (regular
  file, at most 1 MiB) apply to flows unchanged.

## Not in this spec

Shared project defaults (`env`, default shell and timeout), looking for the project in parent
folders, a project-name prefix for registry names, registering new flows automatically, and the
Desktop file picker showing `.truss`. `truss cache clear` is unchanged: it empties the daemon's
cache folder only, not `.truss/state/<flow>/data/cache`. Each is documented as future work.

## Known limitations

- A pipeline run by path whose file stem equals a flow name shares `.truss/state/<stem>/` with that flow.
- On a case-insensitive filesystem, stems that differ only by case share one state folder.
- Names are compared without case on macOS and Windows by operating system, not by volume.
- A pipeline run by path whose file stem is a reserved device name such as `CON`, `CONIN$` or
  `CONOUT$`, or ends in a dot or space, cannot run, because no state folder can be made.
- Links, folders and cloud-sync placeholder files in `.truss/flows` are ignored without a message.
- `truss run --help` is read as a flow name. Open question: should it print help?

## Success criteria

- **SC-001** In a project with two flows, `truss run <name>` runs the named one and writes
  `.truss/state/<name>/run.log`.
- **SC-002** No file outside `.truss/state/` and `.truss/.gitignore` is created by a run.
- **SC-003** A planted link at `.truss`, `.truss/flows`, `.truss/state` or `.truss/state/<flow>` is
  refused and nothing is written through it. A link (or any other existing file) at `.truss/.gitignore`
  is never written through; the run goes on, as FR-015 says an existing file is never changed.
- **SC-004** `docs/pipeline-reference.md`, `docs/cli.md`, `docs/daemon.md`, the README and
  `docs/architecture.md` describe the new layout, and every complete file example in them validates (fragments are not complete files).

## Verification

- `internal/project` tests: metadata and its errors (no value echoed), no lookup in parent folders,
  links and oversized files refused, flow listing (case and `.yaml`/`.yml` clashes, device names,
  the 256-entry cap), state paths, and a named pipe refused without hanging (Unix).
- `cmd/truss/layout_test.go`: run by name, by path and with no argument; a run writes only
  `.truss/state/` and `.truss/.gitignore`; `.truss/.gitignore` is never overwritten or written through
  a link; a failed write warns and the run goes on; `validate` for the project and each flow;
  `register` by name, by path and with `--all`, including the summary line.
- `internal/daemon` tests: a flow runs in the project root, and a flow without `truss.yaml` runs in
  its own folder. `.github/scripts/e2e-test.sh` runs the built binary through the project layout.
- Not yet proved: Windows path cases (CI on Windows is the first proof). Behavior on a
  case-sensitive macOS volume is proved by code review only: Linux compares names exactly, and the
  macOS and Windows runners use case-insensitive volumes, where the test for it skips, so no CI job
  would catch a regression there. A test on a case-sensitive APFS disk image is a follow-up.
- Reviews: an independent review ran on `16c474e`, a delta review on `9574d33` and a final delta
  review on `b6a8509` (reviewer, spec-reviewer, security-reviewer, portability-reviewer as the
  changed paths required). The pull request states the reviewed commit and the final `HEAD`.
