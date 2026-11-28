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
  safe), template, then flow name (`build`, `backup` or `hello` by the chosen template; a `.yaml`
  ending is dropped), template suggested from (`go.mod`,
  `package.json`, `pyproject.toml`/`requirements.txt`/`setup.py`, `Makefile`; else `shell`), how it
  starts (by hand; a schedule: every hour, every day at 02:00, weekdays at 09:00, Mondays at 07:00,
  or a cron expression that is asked again until valid; or when files change, with a path that is
  asked again until valid), and a desktop notification on failure. A choice is a number or the
  option's first word.
- **FR-003** `--yes`, or a standard input that is not a terminal, takes every suggestion. `--name`,
  `--flow` and `--template` set the suggestion.
- **FR-004** The flow file carries short comments on what to change and what to run next. It is
  validated before anything is written; the flow file is written first and `truss.yaml` last, and
  a failure part way removes what was written. Ctrl-C during the questions writes nothing and
  says so. Then it prints the next commands.
- **FR-005** `truss flows new [<name>]` asks the flow questions for an existing project.
- **FR-006** Templates: `go`, `node` (`npm ci` with a lockfile, else `npm install`), `python`
  (`pip install -r requirements.txt`, else `pip install .`), `make`, `backup` (the host shell's
  copy command, which the file says), `shell`.

## Acceptance

`cmd/truss/initcmd_test.go`: init with suggestions writes files that validate and refuses a second
time; flows new adds one and refuses an existing name and an unknown template; scripted answers for
a schedule, a notification, a file hook and a custom cron expression become valid flows.
