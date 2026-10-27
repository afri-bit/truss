# Implementation Plan: Step Dependencies

**Branch**: `003-step-dependencies` | **Date**: 2026-10-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-step-dependencies/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

A Step may list the Step ids that must succeed before it starts, using the optional `needs` field on schema version `"1"`. `internal/pipeline` parses that field, rejects an unknown id, a self-dependency, a duplicate, a bad shape, and a cycle before any command starts, and builds a sequential execution order. When several Steps are ready, the one written earlier in the file is next. The engine keeps its current loop: it runs that order one Step at a time and returns on the first failure, so a ready Step that does not need the failed Step still does not start. The run log and CLI keep their current text. Steps appear in the order they started, and the log does not print `needs`. A definition that does not use `needs` stays in definition order.

## Technical Context

**Language/Version**: Go 1.27.3 (module `github.com/afri-bit/truss`)
**Primary Dependencies**: Go standard library; `gopkg.in/yaml.v3` for YAML decoding only
**Storage**: N/A
**Testing**: `go test ./...`; contract tests for schema validation, execution order, fail-fast, and the run log; test-first per constitution Principle III
**Target Platform**: Local host on Linux, macOS, and Windows. Dependency rules do not vary by host. Shell syntax follows the host shell.
**Project Type**: CLI + engine (daemon is out of scope)
**Performance Goals**: N/A (one local sequential run; no throughput target)
**Constraints**: Only Step `run` strings are executed. Exit Codes stay on the step. A failed Step starts nothing further. `needs` carries Step ids, not secrets. Schema version stays `"1"`. Cancellation, timeouts, retries, and concurrency stay undefined. `context.Context` remains the first parameter of blocking calls and is not stored on structs. No new module dependency.
**Scale/Scope**: One pipeline file, one sequential host run, optional per-step `needs`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] Spec exists; this plan does not invent unspecified behavior
- [x] Core execution remains independent from CLI/presentation
- [x] External contracts do not leak Go-specific types or conventions
- [x] Process execution, cancellation, exit codes, and secret handling are explicit where touched
- [x] Behavioral changes are test-first: a failing test exists before implementation. Tests cover specified behavior (`go test`); integration/contract tests are included when execution or public contracts change
- [x] Public contract changes identify compatibility impact
- [x] No speculative packages, layers, frameworks, or dependencies
- [x] Cross-platform behavior is specified for paths, processes, signals, or shells
- [x] Complexity Tracking records any principle violation and the simpler alternative rejected

Violations MUST be resolved or justified in Complexity Tracking before Phase 0 proceeds.

Post-design re-check (2026-10-27): all gates still pass. Declared edges live on `pipeline.Step`. Validation and the sequential order live in `internal/pipeline`. The engine still executes `ExecutionPlan.Steps` in slice order and stops on the first failure. It does not grow a scheduler, and it does not import `runlog`. The CLI already prints `error: <diagnostic>` and the run log, so it needs no new command. No package and no dependency are added. Cancellation stays out of scope. The unreleased definition-order rule is replaced, only when `needs` is used, by [contracts/execution.md](./contracts/execution.md). Schema version `"1"` stays. Compatibility is recorded in [contracts/pipeline-v1.md](./contracts/pipeline-v1.md) and [contracts/run-log.md](./contracts/run-log.md).

## Project Structure

### Documentation (this feature)

```text
specs/003-step-dependencies/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── pipeline-v1.md
│   ├── execution.md
│   └── run-log.md
└── tasks.md              # created by /speckit.tasks, not this command
```

### Source Code (repository root)

```text
cmd/
└── truss/                # unchanged wiring; contract tests for order, rejection, and the log
internal/
├── pipeline/             # parse needs, validate the graph, plan the sequential order
├── engine/               # existing fail-fast loop over the plan; tests for order and stop
├── executor/             # unchanged host shell
└── runlog/               # unchanged line format
```

Tests live in `*_test.go` beside the package.

**Structure Decision**: `needs` is parsed and checked in `internal/pipeline`, next to the schema rules that already reject a bad definition before execution. `Plan` returns the same `ExecutionPlan` type, with `Steps` arranged in the order FR-006 requires. Declared `Needs` stay on each Step; the tie-break is only the slice order, so a later concurrent feature can read `Needs` without inheriting a fake edge. The engine loop is already "run the plan, stop on failure," which is FR-008. `internal/runlog` does not learn about dependencies. No `internal/graph` package is added: the spec's sequential rule is one function, and a separate package would exist only for a future concurrent scheduler.

## Complexity Tracking

No constitution violations.
