# 0020. Debugging shows values and runs a slice of a flow

- Status: Proposed
- Date: 2026-11-19

## Context

The maintainer made debugging the first priority of v0.2.0: look at one step, see what it needs,
and run just that step with chosen inputs. [ADR 0019](0019-the-cli-grows-by-read-only-commands-and-ascii-screens.md)
item 6 says the CLI prints environment names, never values. Debugging needs the values. The CLI,
the pipeline's process execution and the handling of secrets are public contracts (constitution V).

## Decision

1. **Inspect is `flows show <flow> <step>`.** It is read-only and needs no daemon. A second
   argument was a usage error before, so no existing command changes (ADR 0019 item 1).
2. **Values are shown with rules.** This amends ADR 0019 item 6 for the step view only
   (`flows show <flow>` still prints names). Values declared in the file are shown: they are
   already in the file. Secrets are `set`, `missing` or `***`, never a value. A name that looks
   sensitive (`TOKEN`, `SECRET`, `PASS`, `KEY`, `AUTH`, `SESSION`, `COOKIE` and a few more) is
   masked wherever it comes from. The values of host variables the command reads appear only with
   `--values`, and never when the name looks sensitive or the value contains a stored secret
   (checked for declared values too, under `--values`). The scan of the command
   for variable names is best effort and says so.
3. **A slice is built from the plan.** `internal/pipeline` turns a flow, a step id and a mode
   (the step, the step and what it needs, the step and what needs it) into a smaller pipeline whose
   steps keep their merged `env`, `secrets`, `dir` and settings. The engine runs it unchanged, so
   timeouts, retries, redaction and cancellation behave as in any run. `needs` outside the slice
   are dropped, so the step starts as if they had succeeded. Only the condition of the job of the
   named step is dropped (naming a step is a decision to run it); every other job in the slice keeps
   its `if`, so `--from` never starts a deploy the flow would skip.
4. **`--env` overrides inside the plan.** An override is applied to the steps of the slice above
   every file layer. The engine's base environment cannot do this: a step's own `env` wins over it.
   A secret name is refused, so a secret value never travels as an argument (spec 012).
5. **A debug run leaves the last real run alone.** It prints its log and does not replace
   `run.log`; it still uses the flow's data and cache folders, so earlier output is there. It does
   not combine with `--track`.
6. **The failure hint.** A failed local run prints the command to run only the failed step, on
   standard error, so a script reading standard output or the log is not affected. It is printed
   as a command only when the flow and the id are plain words (no comma, nothing PowerShell or zsh read specially at the start): shells quote differently and a step
   id is free text, so quoting for one shell would let another run code from a file the user did
   not read. Otherwise it names `--step` and leaves the command out.

## Consequences

- Good: no change to how the engine runs steps and no new dependency; a slice function and a record
  of where each variable comes from in `internal/pipeline`, an exported `engine.StepDir` so the step
  view shows the directory a run would use, and some CLI.
  A debug run is exactly a run, so it cannot behave differently from the real thing.
- Good: the same slice and the same step view are what a later `--shell`, `--break`, MCP tool or
  runner override build on.
- Bad: the names `--step`, `--from`, `--with-needs` and `--env` become options of `run`, so a file
  named like one needs `./`. `--track` already works this way.
- Bad: masking by name is a heuristic. It prevents accidents (a pasted screenshot), not a hostile
  file: a file can print any variable from a `run` command, as it always could.
- Bad: reading `$NAME` out of command text misses names a script builds and can list names a
  script sets in a way the scan does not see.
- Follow-ups: `--shell`, `--break`, `--dry-run`, resume from the last failure, `--log`, the last
  result of a step, and exercising `if: failure()` steps.
