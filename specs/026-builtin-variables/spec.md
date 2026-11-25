# Feature Specification: Built-in variables, `${NAME}` and a project-wide env

**Spec**: `026-builtin-variables` | **Created**: 2026-11-25 | **Status**: Implemented

Extends [`005-jobs-schedules-and-step-settings`](../005-jobs-schedules-and-step-settings/spec.md)
(`env`, `dir`), [`008-workspace-and-cache`](../008-workspace-and-cache/spec.md),
[`016-project-layout`](../016-project-layout/spec.md) (the project file) and
[`023-step-debugging`](../023-step-debugging/spec.md) (the step view). User docs:
[`docs/pipeline-reference.md`](../../docs/pipeline-reference.md#built-in-variables).
Decision: [ADR 0023](../../docs/decisions/0023-variables-expand-in-truss-not-in-a-template-language.md).
Risk class: tier 2 (the flow file schema, process environment). Scope set by the maintainer on
2026-11-25 (milestone v0.3.0): "it should be the env variable but we have some reserved keywords".

## Why

A step could not tell which flow, step or attempt it was, a local run had no run id, and a value
could not use another (`${HOME}/data`), so the same path was written in every flow. There was no
way to define a variable once for a project.

## User stories

1. **Know where I am** (P1). A step reads `TRUSS_FLOW`, `TRUSS_STEP`, `TRUSS_RUN_ID` and the
   others in every run, local or daemon.
2. **Define once, use everywhere** (P1). I put `env` in `truss.yaml` and every flow gets it; a flow
   builds on it with `${NAME}`.
3. **Paths from variables** (P1). `dir: ${INBOX}` and `PATH: ${TRUSS_PROJECT_DIR}/bin:${PATH}` work.
4. **A scratch folder** (P2). `TRUSS_TEMP_DIR` is mine for one run and gone after it.

## Requirements

- **FR-001** Every run sets `TRUSS_FLOW`, `TRUSS_PIPELINE` (same value), `TRUSS_OS`, `TRUSS_ARCH`,
  `TRUSS_RUN_ID`, `TRUSS_TRIGGER`, `TRUSS_PROJECT_DIR` and `TRUSS_TEMP_DIR`, and per step
  `TRUSS_STEP` (the id as written), `TRUSS_JOB` (for a step of a job) and `TRUSS_ATTEMPT` (1, 2, ...).
  A local run's id is `local-<YYYYMMDD-HHMMSS>-<6 hex>` unless it is tracked; its trigger is `local`.
  Built-ins win over a variable of the same name inherited from the caller.
- **FR-002** The names of FR-001 are reserved: a flow file's `env` (any level), the project file's
  `env` and a `secrets` list may not use them, in any letter case; validation says so.
  `TRUSS_DATA_DIR` and `TRUSS_CACHE_DIR` stay settable. Other `TRUSS_` names are not reserved.
- **FR-003** `truss.yaml` may hold `env`, with a flow file's rules. It is the lowest layer, under
  the flow's `env`, for every flow of the project, for local and daemon runs.
- **FR-004** `${NAME}` in an `env` value (project, flow, job, step), a `dir` or `workdir` is replaced
  when the step starts. Layers resolve lowest first. Inside a layer a value may use another name of
  that layer; `${NAME}` in the value of `NAME`, and a name the layer does not set, take the value
  from below (lower layers, built-ins, then the run environment). Unset is empty. A loop fails the
  step before it starts, with a diagnostic and no exit code. `$$` is `$`; any other `$` is kept.
  Secrets are added after expansion and are never expanded into another variable. `--env` values
  are taken as written.
- **FR-005** A `${` without `}` or a `${...}` that is not a variable name is a validation error. The
  message never quotes the value.
- **FR-006** `TRUSS_TEMP_DIR` is a new folder per run, removed when the run ends; if it cannot be
  made the run goes on without the variable.
- **FR-007** `truss flows show <flow> <step>` lists the built-ins: known ones with their value
  (`SOURCE` `truss`, `STATE` `set`), per-run ones as `each run` (JSON `each-run`), and `project` as
  a source of a variable.

## Compatibility

- A v0.2.0 file that sets one of the reserved names, or holds `${` or `$$` in an `env` value or a
  `dir`, changes: the first is now an error, the second is now expanded.
- `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and `TRUSS_TRIGGER` were daemon-only; they are now set in
  local runs too. The step view's state `daemon-only` (spec 023) is replaced by `each-run`, and the
  source `daemon` by `truss`.
- The run log and the API are unchanged.

## Acceptance

Tests in `internal/pipeline/expand_test.go`, `internal/engine/builtin_test.go` and
`cmd/truss/builtin_test.go`: every built-in in a job step; the temp folder removed; expansion across
project, flow and step with `$$`; a loop fails the step; a local run's id, trigger, project dir and
project env; reserved names rejected in a flow and a project file; `--env` taken as written.
