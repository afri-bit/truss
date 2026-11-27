# Feature Specification: Step debugging

**Spec**: `023-step-debugging` | **Created**: 2026-11-19 | **Status**: Implemented

Extends [`022-cli-enrichment`](../022-cli-enrichment/spec.md) (`flows show`),
[`001-host-pipeline-engine`](../001-host-pipeline-engine/spec.md) and
[`016-project-layout`](../016-project-layout/spec.md) (`run`, state folders), and
[`012-secrets`](../012-secrets/spec.md). User docs: [`docs/cli.md`](../../docs/cli.md) and
[`docs/cli-guide.md`](../../docs/cli-guide.md).
Decision: [ADR 0020](../../docs/decisions/0020-debugging-shows-values-and-runs-a-slice.md).
Risk class: tier 2 (CLI usage and output, process execution, secrets handling). Scope set by the
maintainer on 2026-11-19: debugging is the highest-priority capability and ships in v0.2.0. Values
in the environment table, and secrets as set or missing, were decided the same day.

## Why

A flow that fails at step 9 of 12 is hard to debug today: the only tool is to run all of it again.
The maintainer wants to look at one step, see everything it needs, and run just that step, or the
steps around it, with inputs of their choice.

## User stories

1. **What does this step need?** (P1). I ask for one step of a flow and see its command, the steps
   before and after it, its settings, and a table of the environment it can use with the value each
   variable has now. Secrets show as set or missing, never their value. A variable the command
   uses but nothing sets is easy to spot.
2. **Run only this step.** (P1). I run one step of a flow, with or without the steps it needs, or a
   step and everything that depends on it. A failure tells me the command to run that step again.
3. **Try other inputs.** (P1). I override environment variables for that run, without editing the
   file.
4. **Leave no trace.** (P1). A debug run does not replace the run log of the last real run.

## Requirements

### Inspect

- **FR-001** `truss flows show <name|path> <step> [--values] [--json]` prints one step of a flow
  and changes nothing (no file, no folder, no daemon call). `<step>` is the step id as `flows show`
  lists it: `job/step` for a step of a job. It prints: the flow, file and project; the step id and
  job; the command (all lines of `run`); the directory it starts in (worked out as a run works it
  out: an absolute `dir` as it is, a relative one joined to the directory the run starts in, and the reason when a run
  would refuse it); the steps it needs and the
  steps that need it; its timeout, retries, retry delay, `continue-on-error` and `if` (and the job's
  `if`); then the environment table (FR-003). Text goes through the same cleaning as the other
  screens (spec 022 FR-014).
- **FR-002** A step id that does not exist is an error (exit `1`) that names the step and lists the
  ids of the flow. A flow that does not load is the error of `flows show`. These errors are plain
  messages on standard error, also with `--json`: no JSON document is printed for them.
- **FR-003** The environment table has one row per variable the step can use, with columns `NAME`,
  `SOURCE`, `STATE` and `VALUE`:
  - **declared**: every variable in `env` at the pipeline, job or step level that reaches the step.
    `SOURCE` is `pipeline`, `job <id>` or `step`, whichever layer wins. `STATE` is `set`. `VALUE` is
    the value from the file.
  - **secret**: every secret the step declares. `SOURCE` is `secret store`. `STATE` is `set` when
    the store holds it, `missing` when it does not (a run would stop before any step started). `VALUE`
    is `***` when set and `-` when missing. A secret's value is never printed, with or without
    `--values`.
  - **truss**: `TRUSS_DATA_DIR` and `TRUSS_CACHE_DIR` (the state folders of this flow, as
    `truss run` sets them: `SOURCE` `truss run`, `STATE` `set`; the text form shows them relative to
    the project, the JSON the absolute path) and `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and
    `TRUSS_TRIGGER` (`SOURCE` `daemon`, `STATE` `daemon only`: only the daemon sets them, `VALUE`
    `-`). *Changed by [spec 026](../026-builtin-variables/spec.md): every run sets them; the table shows `SOURCE` `truss`, `STATE` `each run` (JSON `each-run`), and the other built-ins with their value.*
  - **referenced**: a whole name in upper case, digits and `_` (starting with a letter or `_`,
    and not `_` alone) that the command reads as `$NAME`, `${NAME}`, `${NAME:-...}`, `%NAME%`
    (two characters or more, so `date +%Y%m%d` adds nothing) or `$env:NAME` (`env` in any case),
    when no other row has that name and the command does not assign it (`NAME=`, `set "NAME=`,
    `$env:NAME =` or `for NAME in`). `$Home` is the name `Home`, which is not upper case, so it is
    not listed. `SOURCE` is `host environment`. `STATE` is `set` or `not set`, from the
    environment of the `truss` process. `VALUE` is `-` unless `--values` is given, which prints
    the value.
  This list is a best-effort reading of the command text, not a shell parser: a variable a script
  builds by other means does not appear, and the table says so in a note.
  A name appears in one row, the row of the layer a run lets win: a secret over a variable of the
  file, and a variable of the file over what `truss run` sets. Names are compared without regard to
  case on Windows, as the host does, and exactly on Linux and macOS.
- **FR-004** A variable whose name contains `TOKEN`, `SECRET`, `PASS`, `CREDENTIAL`, `PRIVATE`,
  `AUTH`, `KEY`, `DSN`, `COOKIE`, `SESSION`, `JWT`, `BEARER` or `WEBHOOK` (any case) is never
  printed: its `VALUE` is `***`, even when it is declared in the file and even with `--values`. With
  `--values`, any value, declared or from the host, that contains a stored secret (not only equals
  one) is printed as `***` too; the stored values are read only for `--values`. `--values` only
  shows the values of referenced host variables; declared values are always shown (they are in the
  file).
- **FR-005** `--json` prints one document: `flow`, `file`, `project` (when the flow is in a
  project), `step` (`id`, `run`, `run_dir` (the directory the step starts in, absolute), `needs`,
  `needed_by`, and, where they apply, `job`, `dir`, `dir_error`, `timeout`, `retries`,
  `retry_delay`, `continue_on_error`, `if`, `job_if`) and `environment`, a list of `{name, source, kind, state, value}` where `kind` is
  `declared`, `secret`, `truss` or `referenced` and `state` is `set`, `not-set`, `missing` or
  `daemon-only` (the text form writes `not set` and `daemon only`). `value` is present only when FR-003 and FR-004 allow printing it. Empty lists are
  `[]`.

### Run a slice

- **FR-006** `truss run <name|path> --step <id>` runs only that step. The steps it needs are not
  run, and the step starts as if they had succeeded. `--with-needs` also runs every step it needs,
  directly or not (the step and everything before it). `--from <id>` runs the step and every step
  that needs it, directly or not. `--from` and `--step` together, and `--with-needs` without
  `--step`, are errors (exit `1`). An unknown id fails as in FR-002 before any step starts.
- **FR-007** The steps of a slice keep their settings: command, `env`, `secrets`, `dir`, `timeout`,
  `retries` and `continue-on-error`. Concurrency follows the flow. Their own `if` is evaluated as
  if the steps they need outside the slice had succeeded. The `if` of the job of the step named
  with `--step` or `--from` is not evaluated: naming a step is a decision to run it. Every other
  job in the slice, one `--from` pulls in after the named step or `--with-needs` pulls in before
  it, keeps its `if`, so a deploy or a cleanup the flow would skip is still skipped.
- **FR-008** `--env NAME=VALUE` (repeatable) replaces or adds an environment variable for the steps
  that run, above every layer of the file. It works with `--step`, `--from` or on its own for a
  whole run. `NAME` must be a valid name (letters, digits, `_`, not starting with a digit); a
  malformed option is an error (exit `1`) that does not repeat the text it was given. A `NAME` that a selected step declares in `secrets` is an
  error that says to use `truss secret set`: a secret value is never an argument (spec 012). Names
  are compared without regard to case on Windows, for this refusal and for which variable wins; of
  two overrides that differ only in case, the later one wins.
  An override is seen by `if` conditions (`env.NAME`), as any variable of the step is. Values are
  not written to the run log, as before. An argument is visible to other users of the machine in
  its process list, which the docs say.
- **FR-009** A run with `--step` or `--from` is a debug run. It prints the run log to standard
  output as a real run does, and does **not** write `.truss/state/<flow>/run.log`: the log of the
  last real run stays. It uses the flow's data and cache folders, so files an earlier run left are
  there. It prints one line to standard error before it starts:
  `debug run: <n> of <total> steps (<ids>); the run log is not saved`. The exit code is the one
  `truss run` has: `0`, or `1` when a step failed or the run was interrupted. A debug run cannot be
  combined with `--track` (an error).
- **FR-010** When a local `truss run` fails and the run knows which step failed first, it prints
  one line to standard error after the summary:
  `to run only this step again: truss run <flow> --step <id>` (`<flow>` as the user wrote it, and
  without the `--env` options the run had). The line is a command to paste, so it is printed only
  when `<flow>` and `<id>` are plain words: letters, digits and `_ . / : @ + = -` (and `\` on
  Windows), not starting with `-`, `@` or `=` (PowerShell and zsh read those specially). Otherwise the line says to use `truss run --step` with the
  step's id, quoted for the shell, and repeats neither. A debug run prints no such line. Nothing
  else about a failed run changes.

## Not in this spec

An interactive shell in a step's environment, step-by-step stepping, `--dry-run`, resuming from the
last failure, saving a debug log (`--log`), the last result of a step in `flows show <step>`,
running a different runner (container), simulating a trigger or event payload other than by
`--env`, exercising a step that only runs `if: failure()`, and debug runs through the daemon.

## Compatibility

Additive, with these precise rules so a script never changes meaning:

- `truss flows show <flow> <step>` printed the usage text (exit `1`) before; `flows show <flow>`
  is unchanged.
- `truss run <x>` behaves as before, apart from the line on failure below, when none of `--step`, `--from`, `--with-needs` or `--env` is given.
  As `--track` does today (spec 001), these four words are options, so a file literally named
  `--step` runs as `truss run ./--step`. The forms `--step=<id>`, `--from=<id>` and
  `--env=NAME=VALUE` are options too; `--track=x` and `--with-needs=x` stay paths.
- A failed local `truss run` gains one line on standard error (FR-010). Standard output, the run
  log, the saved `run.log` and the exit code are unchanged.
- The run-log format is unchanged.

## Platforms

Pure text and process work with the existing executor: no platform file. Paths use `filepath`,
so the text form shows the separators of the host. `%NAME%` and `$env:NAME` are read on every
platform (it is only text). Environment variable names are compared without regard to case on
Windows only. Tests run on Linux, macOS and Windows in CI.

## Testing

`internal/pipeline` tests cover the slice (`--step`, `--with-needs`, `--from`, unknown ids, jobs, the
override, the source of each variable). `cmd/truss` tests drive `flows show <step>` and `run`
through `Run`: the table, the masking, that nothing is created, that `run.log` is untouched by a
debug run, the exit codes, the hint and the compatibility rules. `.github/scripts/e2e-test.sh` runs
the built binary through `flows show <step>` and `run --step`.
