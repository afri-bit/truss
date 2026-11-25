# Feature Specification: One vocabulary, "flow"

**Spec**: `024-flow-vocabulary` | **Created**: 2026-11-25 | **Status**: Implemented

Decision: [ADR 0021](../../docs/decisions/0021-flow-is-the-word.md). Risk class: tier 2 (the flow
file schema, CLI output). Scope set by the maintainer on 2026-11-25 ([milestone](../../docs/MILESTONE.md),
v0.3.0): the user-facing word is "flow".

## Why

TRUSS called the same thing a pipeline (file key, CLI, docs) and a flow (folder, Desktop, newer
commands). New users asked whether they were two things. One word removes the question.

## User stories

1. **One word** (P1). As a user I read "flow" in the file, the help, the command output, Desktop and
   the docs, and nowhere a second word for the same thing.
2. **Nothing breaks** (P1). As a user of v0.2.0 my files, scripts and API clients keep working
   unchanged.

## Requirements

- **FR-001** A flow file accepts the block `flow:` with the fields the `pipeline:` block has
  (`name`, `concurrency`, `workdir`, `overlap`). `pipeline:` stays accepted with the same meaning.
- **FR-002** A file with both `flow:` and `pipeline:` is invalid; the error names both and says to
  keep one.
- **FR-003** Validation messages, CLI help, CLI prose output and Desktop labels say "flow" where
  they said "pipeline". Examples: `flow concurrency must be ...`, `unknown field "x" in flow`,
  `<file> is valid: flow <name>, ...`, `registered 2 flows (1 with schedules)`, the `Flow` row of
  `truss runs show` and `truss pipelines show`, the daemon log lines `flow <name>: ...`.
- **FR-004** `truss runs show last --flow <name>` selects the newest run of a flow. `--pipeline`
  stays accepted; giving both is an error.
- **FR-005** These keep their names, because they are released contracts: the `truss pipelines`
  command, the daemon API paths and JSON fields (`/pipelines`, `pipeline`, `registered_pipeline`),
  the environment variable `TRUSS_PIPELINE`, the `source` value `pipeline` in
  `truss flows show <flow> <step> --json`, and the home folder `~/.truss/pipelines/`. The step
  table shows that source as `flow`.
- **FR-006** The glossary defines Flow, and Pipeline as its older name; user docs use "flow".

## Compatibility

Every v0.2.0 flow file stays valid, including one that repeats the `pipeline:` block (the last
one wins, as before). JSON field names, the API paths and the run log are unchanged.
Human-readable text changes wording (FR-003): table headers, prose output, and the English text
of errors and run reasons, which also reach JSON as values (`error`, `reason`). Runs recorded
before keep their old reason text. A script that matches English text, such as
`registered 2 pipelines` or the `PIPELINE` column, must change; scripts should read `--json`.

This spec changes wording that earlier specs quote. They are not rewritten; this list is the
record: [004](../004-step-concurrency/spec.md) (validation messages: `flow concurrency ...`,
`unknown field ... in flow`), [008](../008-workspace-and-cache/spec.md) (`flow workdir ...`),
[016](../016-project-layout/spec.md) (`no flows`, `flows live in .truss/flows/`,
`registered N flows`), [022](../022-cli-enrichment/spec.md) (`--flow`, the `Flow` row, the `FLOW`
column) and [023](../023-step-debugging/spec.md) (the step table shows the source `flow`).

## Acceptance

- A file with `flow:` and the same file with `pipeline:` produce the same plan (test
  `TestParse_FlowKeyIsTheFlowBlock`).
- A file with both is rejected, in either order (`TestValidate_RejectsBothFlowAndPipeline`,
  `TestValidate_RejectsBothInEitherOrder`); a repeated block of one name stays valid
  (`TestValidate_ARepeatedBlockOfOneNameStaysValid`).
- The end-to-end script runs a file that uses `flow:` (`flow key`).
- `--flow` and `--pipeline` select the same run; both together fail (`TestRuns_*`).
