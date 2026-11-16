# 0015. Project layout

- Status: Proposed
- Date: 2026-11-15

## Context

One `truss.yaml` per folder made several pipelines per project impossible by convention. Also,
`.truss/` mixed generated state (gitignored) with files worth committing. Two blind consults on
2026-11-15 (architect, security-reviewer) agreed on the core. They are in the work log
(`feature--016-project-layout`).

## Decision

**Project file.** The root `truss.yaml` holds project metadata only: `version: "1"`, `project.name`
(required) and `project.description` (optional). Fields are strict. `steps` or `jobs` there is an
error that points to `.truss/flows/`. The error never echoes values.

**Pipelines.** Each pipeline lives in `.truss/flows/<name>.yaml` or `.yml`. "Flows" is the folder
name only. The term is still Pipeline.

**Project root.** The project root is the working directory. There is no walk-up in v0.1.0. A
planted parent `truss.yaml` must not redirect where commands run or where state is written. Walk-up
can be added later without breaking anyone. Removing it later could not be done safely.

**Arguments.** For `run`, `validate` and `register`, an argument that contains `/` or `\`, or ends
in `.yaml` or `.yml`, is a path. Any other argument is a flow name. TRUSS never falls back from one
to the other. A name follows the registry's name rule, and it has no trailing dot or space and no
Windows device name.

**Pipeline name.** `pipeline.name` is optional and defaults to the file stem. `validate` notes a
mismatch.

**Discovery.** `.truss` and `.truss/flows` must be real directories, checked with Lstat. Entries
must be regular files: no links, no subfolders. Hidden files are ignored. Extensions are `.yaml` or
`.yml`, case-sensitive. Stems that differ only by case or by extension clash, which is an error.
A file is capped at 1 MiB and is read through an open handle (fstat on the handle, limited reader).
A folder holds at most 256 entries. A broken flow is an error, never skipped.

**State.** Generated files move to `.truss/state/<stem>/`: `run.log`, `data/` and `data/cache/`.
These folders are created with mode 0700. `.truss/.gitignore`, containing only `state/`, is created
with O_EXCL when state is first made. It is never overwritten and never written through a link. If
it cannot be written, TRUSS warns and the run goes on. Old `.truss/run.log` and `.truss/data` are
ignored. The CHANGELOG notes this.

**Register.** `register` takes a name, a path or `--all`. There is no implicit bulk registration. A
repository is a weaker trust root than a file the user picked. The registry stays name to path.

**Daemon working directory.** Steps of a flow in `<root>/.truss/flows/` run in `<root>`. The new
`internal/project` package finds `<root>`, checks the structure with Lstat and requires `truss.yaml`
there. Any other file runs in its own directory, as it does today.

**Package.** `internal/project` owns finding the project, parsing the metadata, listing flows,
resolving a name and the state paths. It runs nothing and prints nothing. `internal/pipeline` stays
unaware of the layout.

## Consequences

- Good: several pipelines per project, with one clear convention.
- Good: committed definitions stay apart from generated state.
- Good: safer reads: no links, no unbounded files, no silent skips.
- Bad: a breaking change to the single-file form. Nothing is tagged yet.
- Bad: the run-log path (spec 002) and the data path (spec 008) change.
- Bad: running from a subfolder finds no project. The error names the folder it looked in.
- Bad: a flow name can clash with another repository's name at `register`. Use `--name`.
- Same PR: `docs/architecture.md` and `.claude/rules/go-architecture.md` change. The repository
  `.gitignore` stops ignoring all of `.truss/`.
- Follow-ups, documented but not built: shared project defaults (env, default shell, timeout);
  walk-up discovery; a project-name prefix for registry names; auto-sync of new flows to the daemon;
  the TRUSS Desktop file picker showing `.truss`.
- Still open: the daemon's `readDefinition` uses `os.Stat`, then a second read. It needs an
  open-then-fstat read with a limit, and a non-blocking open for named pipes. The 1 MiB cap already
  applies in the daemon, and `pipeline.ParseFile` reads through the open handle with a limit.
- Still open: registry paths compared across case and 8.3 short-name spellings.
- Still open: a Windows junction test.
- Still open: drive-relative arguments such as `C:foo.yaml`.
- Still open: `register --all` and scheduled pipelines (what runs after a `git pull`).
- Still open: case folding of `.truss`, `flows` and `truss.yaml` decides by operating system, not by
  volume. A case-sensitive APFS volume folds; Linux with casefold folders does not.
- Still open: the check-then-open race on named pipes. The non-blocking open is not yet used for the
  pipeline, project and daemon reads.
