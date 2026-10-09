# Implementation Plan: Host-Executed Pipeline Engine

**Branch**: `001-host-pipeline-engine` | **Date**: 2026-10-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-host-pipeline-engine/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Load a v1 YAML Pipeline Definition, validate it, run each Step's `run` string sequentially through the host shell, and return a structured Result through a thin `truss run` CLI. An explicit path is used as given. With no path, the CLI loads `truss.yaml` from the invoker's working directory. Each Step starts in that working directory and inherits the invoker's environment. A non-zero Exit Code or a command that cannot start fails the Step and the Pipeline Run; later Steps do not run.

## Technical Context

**Language/Version**: Go 1.22.5 (module `github.com/afri-bit/truss`)
**Primary Dependencies**: Go standard library; `gopkg.in/yaml.v3` for YAML decoding only
**Storage**: N/A (Pipeline Definitions are local files; Pipeline Runs are not persisted)
**Testing**: `go test ./...`; contract tests for YAML v1, host execution, and `truss run`; test-first per constitution Principle III
**Target Platform**: Local host (Unix-like and Windows). The same Execution Semantics apply; shell syntax follows the host shell.
**Project Type**: CLI + engine (daemon is out of scope)
**Performance Goals**: N/A (one local sequential run; no throughput target in this milestone)
**Constraints**: Only Step `run` strings are executed. Exit Codes are preserved. Fail-fast. No environment dumps. No cancellation, timeout, or retry feature. `context.Context` is the first parameter of blocking calls and is not stored on structs.
**Scale/Scope**: One Pipeline, one or more Steps, sequential host execution, human-readable CLI Result

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

Post-design re-check (2026-10-09): all gates still pass. Cancellation remains out of scope; `context.Context` is plumbed so subprocess calls can be bound to the call, and the CLI uses a background context. No `cancelled` status is introduced. The YAML dependency is justified in [research.md](./research.md).

## Project Structure

### Documentation (this feature)

```text
specs/001-host-pipeline-engine/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── pipeline-v1.md
│   ├── host-execution.md
│   └── cli.md
└── tasks.md              # created by /speckit.tasks, not this command
```

### Source Code (repository root)

```text
cmd/
└── truss/                # CLI: resolve path, print Result, process exit code
internal/
├── pipeline/             # model, YAML parse, validation, sequential plan
├── engine/               # run a plan; no CLI formatting
└── executor/             # host shell execution only
```

Tests live in `*_test.go` beside the package. Pipeline fixtures live in `testdata/`.

**Structure Decision**: Three internal packages match the spec boundaries that must stay independently testable: definition handling (`pipeline`), orchestration (`engine`), and host process execution (`executor`). Parse and validation stay inside `pipeline` rather than extra packages. The daemon, container executor, and remote executor are not created. `cmd/truss` imports the engine and prints its Result; the engine does not import `cmd/truss`.

## Complexity Tracking

No constitution violations.
