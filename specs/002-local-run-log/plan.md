# Implementation Plan: Local Run Log

**Branch**: `002-local-run-log` | **Date**: 2026-10-26 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-local-run-log/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

`truss run` keeps sequential host execution and adds a run log the developer can read live and open afterward. Before any step output, the log records that the environment is ready, the operating system, the architecture, the host shell, and the absolute working directory. Every line TRUSS writes carries a local timestamp. Each started step also shows its start, its output as produced, its finish, its duration, and its Exit Code or start error. The log closes with the pipeline status, the number of steps that started, the run duration, and the time the run finished. The same bytes are saved to `.truss/run.log` in the invoker's working directory, replacing the previous file only after a run has started and finished. Help and version print without starting a run. No environment variables or secrets are added to the log.

## Technical Context

**Language/Version**: Go 1.27.3 (module `github.com/afri-bit/truss`)
**Primary Dependencies**: Go standard library; `gopkg.in/yaml.v3` for YAML decoding only
**Storage**: N/A
**Testing**: `go test ./...`; contract tests for the run log text, `.truss/run.log`, and the CLI commands; test-first per constitution Principle III
**Target Platform**: Local host on Linux, macOS, and Windows. Shell syntax follows the host shell.
**Project Type**: CLI + engine (daemon is out of scope)
**Performance Goals**: N/A (one local sequential run; no throughput target)
**Constraints**: Only Step `run` strings are executed. Exit Codes stay on the step. Fail-fast stays. The setup section and summary list no environment variable names or values and no secrets. Timestamps are local `YYYY-MM-DD HH:MM:SS` and do not move backward. Duration stays beside timestamps, in seconds with three decimal places. Cancellation, timeouts, and retries stay undefined. `context.Context` remains the first parameter of blocking calls and is not stored on structs. This feature does not edit ignore files and does not add pipeline fields.
**Scale/Scope**: One pipeline, one latest run log, sequential host execution, help, and version

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

Post-design re-check (2026-10-26): all gates still pass. The run log text lives in `internal/runlog`. The engine accepts a small step-event interface and does not import `runlog` or format the transcript. The CLI owns help, version, `.truss/run.log`, and the setup and summary lines. No new dependency is added. Cancellation stays out of scope. The unreleased transcript from `specs/001-host-pipeline-engine/contracts/cli.md` is replaced by [contracts/run-log.md](./contracts/run-log.md) and [contracts/cli.md](./contracts/cli.md). Schema version `"1"` is unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/002-local-run-log/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── run-log.md
│   └── cli.md
└── tasks.md              # created by /speckit.tasks, not this command
```

### Source Code (repository root)

```text
cmd/
└── truss/                # arguments, help, version, .truss/run.log, process exit
internal/
├── pipeline/             # unchanged schema, parse, validate, plan
├── engine/               # sequential fail-fast run; emits step events; no transcript text
├── executor/             # host shell; exposes the shell path the log reports
└── runlog/               # timestamped run-log lines and durations
```

Tests live in `*_test.go` beside the package.

**Structure Decision**: `internal/runlog` exists because the live transcript and the saved file must be the same bytes, and that text is a presentation contract. The engine does not import it. `cmd/truss` validates the definition, prepares `.truss`, writes setup, calls the engine with the run log as the step-event sink, writes the summary, and replaces `.truss/run.log` only after the run finishes. Parse, validation, planning, and host execution stay in their current packages. No daemon, container executor, or remote executor is added.

## Complexity Tracking

No constitution violations.
