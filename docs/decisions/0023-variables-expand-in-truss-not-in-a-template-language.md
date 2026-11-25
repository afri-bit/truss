# 0023. Variables expand in TRUSS, not in a template language

- Status: Proposed
- Date: 2026-11-25

## Context

Users want to define a value once and build on it (`${HOME}/data`), and to know where a step runs
(flow, step, run id). The maintainer asked for "the env variable, with some reserved keywords"
(milestone v0.3.0, 2026-11-25). The flow file and the process environment are public contracts
(constitution V). A template language like `{{ }}` or `${{ }}` would be a second syntax to learn.

## Decision

1. Shell-like `${NAME}` is the only syntax, in `env` values and directories. `$$` escapes.
   `$NAME` without braces is left for the step's shell.
2. Expansion is done by TRUSS when the step starts, layer by layer (project, flow, job, step), so a
   value can extend the one below (`PATH: x:${PATH}`). Secrets are never expanded.
3. A fixed list of built-in names is reserved; the `TRUSS_` prefix as a whole is not, because users
   already use it for their own variables and secrets.
4. `truss.yaml` gains `env` for project-wide variables.

## Consequences

- One syntax, close to what a shell does; values are portable across shells because TRUSS, not the
  shell, expands them.
- Files that held a literal `${` or `$$` in a value change meaning; the changelog says so.
- New built-ins later may clash with a user's `TRUSS_` variable; the docs advise another prefix.
