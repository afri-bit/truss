# Feature Specification: `truss init`

**Spec**: `031-truss-init` | **Created**: 2026-11-27 | **Status**: Implemented

Extends [`016-project-layout`](../016-project-layout/spec.md). User docs:
[`docs/getting-started.md`](../../docs/getting-started.md), [`docs/cli.md`](../../docs/cli.md).
Risk class: tier 1 (a new command that writes files; no contract changes). Scope set by the
maintainer on 2026-11-25 (milestone v0.3.0): "a wizard to set up the pipeline so users won't need
to read 1000 pages of documentation".

## User stories

1. **Start in a minute** (P1). In my Go project I run `truss init`, press Enter a few times, and
   `truss run build` works.
2. **Start it on a schedule** (P1). I pick "every day at 02:00" and the flow has the cron line.
3. **More flows later** (P2). `truss flows new backup` adds one.
4. **Scripts** (P2). `truss init --yes` in a CI or a script asks nothing.

## Requirements

- **FR-001** `truss init` writes `truss.yaml` and `.truss/flows/<flow>.yaml` in the working
  directory. It refuses a folder that has a `truss.yaml` and never overwrites a file.
- **FR-002** Questions, each with a suggestion taken with Enter: project name (the folder name made
  safe), flow name (`build`, `backup` or `hello` by template), template (suggested from `go.mod`,
  `package.json`, `pyproject.toml`/`requirements.txt`/`setup.py`, `Makefile`; else `shell`), how it
  starts (by hand; a schedule: every hour, every day at 02:00, weekdays at 09:00, Mondays at 07:00,
  or a cron expression that is checked until valid; or when files change, with a path that is
  checked), and a desktop notification on failure.
- **FR-003** `--yes`, or a standard input that is not a terminal, takes every suggestion. `--name`,
  `--flow` and `--template` set the suggestion.
- **FR-004** The flow file carries short comments on what to change and what to run next, and is
  validated before the command says it is done; then it prints the next commands.
- **FR-005** `truss flows new [<name>]` asks the flow questions for an existing project.
- **FR-006** Templates: `go`, `node`, `python`, `make`, `backup` (with the host shell's copy
  command), `shell`.

## Acceptance

`cmd/truss/initcmd_test.go`: init with suggestions writes files that validate and refuses a second
time; flows new adds one and refuses an existing name and an unknown template; scripted answers for
a schedule, a notification, a file hook and a custom cron expression become valid flows.
