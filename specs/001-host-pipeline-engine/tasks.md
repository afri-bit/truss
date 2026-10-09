# Tasks: Host-Executed Pipeline Engine

**Input**: Design documents from `/specs/001-host-pipeline-engine/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED. Write each story's tests first and confirm they fail before that story's implementation. Contract tests cover YAML v1, host execution, and `truss run`.

**Organization**: Tasks are grouped by user story so each story can be implemented, tested, and demonstrated on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label ([US1], [US2], [US3])
- Every task includes a file path

## Path Conventions

- Go module `github.com/afri-bit/truss`
- `cmd/truss/` CLI only
- `internal/pipeline/` model, parse, validate, plan
- `internal/engine/` orchestration
- `internal/executor/` host shell
- Tests in `*_test.go` beside the code; fixtures in `internal/pipeline/testdata/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Module dependency and package boundaries from plan.md

- [X] T001 Add `gopkg.in/yaml.v3` to `go.mod` and `go.sum`
- [X] T002 [P] Add package docs for pipeline, engine, and executor responsibilities in `internal/pipeline/doc.go`, `internal/engine/doc.go`, and `internal/executor/doc.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared types every story uses. No parse, validation, or execution behavior in this phase.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 [P] Define `Pipeline`, `Step`, `ExecutionPlan`, `PipelineRun`, `StepRun`, and `Status` (`succeeded`, `failed`) in `internal/pipeline/model.go` per `specs/001-host-pipeline-engine/data-model.md`
- [X] T004 [P] Define the host execution request and result (run string, working directory, environment, exit code, stdout, stderr, start error) in `internal/executor/result.go`

**Checkpoint**: Types exist and compile. User stories can begin.

---

## Phase 3: User Story 1 - Run a valid local pipeline (Priority: P1) 🎯 MVP

**Goal**: Load a valid YAML pipeline, run its Steps in order on the host, and print a succeeded Result.

**Independent Test**: Run a two-Step definition whose commands succeed. Both Step Runs appear in definition order with status `succeeded`, Exit Code 0, and captured output, and the Pipeline Run succeeded.

### Tests for User Story 1 (REQUIRED)

> Write these tests first and confirm they fail before T010–T014.

- [X] T005 [P] [US1] Add a contract test that `internal/pipeline/testdata/valid.yaml` parses to version `"1"`, the pipeline name, and Steps in file order in `internal/pipeline/parse_test.go`
- [X] T006 [P] [US1] Add a test that a valid model becomes a definition-order Execution Plan in `internal/pipeline/plan_test.go`
- [X] T007 [P] [US1] Add a host contract test that a successful shell command returns Exit Code 0 and stdout in `internal/executor/host_test.go` (Unix `echo`, Windows `echo`)
- [X] T008 [P] [US1] Add an engine test that two successful Steps produce a succeeded Pipeline Run in definition order in `internal/engine/engine_test.go`
- [X] T009 [P] [US1] Add CLI contract tests for `truss run <path>`, a filename other than `truss.yaml`, and bare `truss run` loading `truss.yaml`, expecting the success report and process exit 0, in `cmd/truss/run_test.go` per `specs/001-host-pipeline-engine/contracts/cli.md`

### Implementation for User Story 1

- [X] T010 [P] [US1] Implement YAML decoding of `version`, `pipeline.name`, and `steps` in `internal/pipeline/parse.go`
- [X] T011 [P] [US1] Implement the sequential Execution Plan in `internal/pipeline/plan.go`
- [X] T012 [P] [US1] Implement host execution with `/bin/sh -c` or `cmd /c`, the captured invoker directory and environment, and stdout/stderr/exit code in `internal/executor/host.go` per `specs/001-host-pipeline-engine/contracts/host-execution.md`
- [X] T013 [US1] Implement `Run` to execute the plan in order and return a succeeded `PipelineRun` in `internal/engine/engine.go`
- [X] T014 [US1] Resolve an explicit path or `truss.yaml`, call the engine, and print the success report from `cmd/truss/run.go`, with `cmd/truss/main.go` delegating to it

**Checkpoint**: User Story 1 passes on its own. A valid local pipeline runs through the CLI.

---

## Phase 4: User Story 2 - Reject invalid definitions without executing (Priority: P2)

**Goal**: Parse and validation errors are visible and start no Step command.

**Independent Test**: For invalid YAML, unreadable files, missing required fields, empty Steps, duplicate IDs, empty `run`, and a missing or unsupported version, the user sees an error and no command runs. Missing `truss.yaml` on a bare `truss run` does the same.

### Tests for User Story 2 (REQUIRED)

> Write these tests first and confirm they fail before T018–T021.

- [X] T015 [P] [US2] Add parse tests for invalid YAML, an empty file, and a missing path in `internal/pipeline/parse_test.go`
- [X] T016 [P] [US2] Add validation tests for missing name, empty Steps, duplicate IDs, empty `id` or `run`, and version other than `"1"` in `internal/pipeline/validate_test.go`, asserting no executor call
- [X] T017 [P] [US2] Add CLI contract tests that invalid files and a missing default `truss.yaml` write `error:` to stderr, exit 1, and print no `step` block in `cmd/truss/run_test.go`

### Implementation for User Story 2

- [X] T018 [US2] Return a user-visible error for unreadable files and invalid YAML without starting a run in `internal/pipeline/parse.go`
- [X] T019 [US2] Implement the rejection rules from `specs/001-host-pipeline-engine/contracts/pipeline-v1.md` in `internal/pipeline/validate.go`
- [X] T020 [US2] Refuse an invalid model in `internal/engine/engine.go` without calling `internal/executor/host.go`
- [X] T021 [US2] Map load and validation failures to stderr and process exit 1 in `cmd/truss/run.go`

**Checkpoint**: User Stories 1 and 2 both pass. Invalid definitions never execute.

---

## Phase 5: User Story 3 - Stop on failure and inspect the failed step (Priority: P3)

**Goal**: A failed Step fails the Pipeline Run, keeps its Exit Code and output, and later Steps do not run.

**Independent Test**: In a three-Step definition, Step 2 exits non-zero. Step 1 succeeded, Step 2 failed with that Exit Code and output, Step 3 is absent and not `succeeded`, and the Pipeline Run is `failed`.

### Tests for User Story 3 (REQUIRED)

> Write these tests first and confirm they fail before T025–T027.

- [X] T022 [P] [US3] Add host tests that a non-zero exit is preserved and a missing program fails to start with a diagnostic and no Exit Code in `internal/executor/host_test.go` (Unix `exit 1`, Windows `exit /b 1`)
- [X] T023 [P] [US3] Add an engine test that a failed second Step omits the third and marks the Pipeline Run failed in `internal/engine/engine_test.go`
- [X] T024 [P] [US3] Add a CLI contract test for the failed report, the omitted later Step, and process exit 1 in `cmd/truss/run_test.go` per `specs/001-host-pipeline-engine/contracts/cli.md`

### Implementation for User Story 3

- [X] T025 [US3] Preserve non-zero Exit Codes and start-failure diagnostics in `internal/executor/host.go`
- [X] T026 [US3] Stop after the first failed Step Run and leave later Steps out of the Result in `internal/engine/engine.go`
- [X] T027 [US3] Print the failed Step report, including `exit_code` or `error`, in `cmd/truss/run.go`

**Checkpoint**: All three stories pass independently. Fail-fast host execution matches the spec.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates that apply to the whole feature

- [X] T028 [P] Walk through `specs/001-host-pipeline-engine/quickstart.md` against the built `truss` CLI
- [X] T029 Run `gofmt`, `go vet`, and `go test ./...` from the module root that contains `go.mod`
- [X] T030 [P] Review `cmd/truss/run.go` and `internal/executor/host.go` so reports contain command output only and do not dump the environment

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup. Blocks all user stories
- **User Stories (Phases 3–5)**: Depend on Foundational
- **Polish (Phase 6)**: Depends on the user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational. No dependency on US2 or US3
- **User Story 2 (P2)**: Starts after Foundational. Independently testable through parse and validate. Extends `internal/pipeline/parse.go`, `internal/engine/engine.go`, and `cmd/truss/run.go` from US1, so implement it after US1 unless those files are split first
- **User Story 3 (P3)**: Starts after Foundational. Independently testable through the engine and host executor. Extends `internal/executor/host.go`, `internal/engine/engine.go`, and `cmd/truss/run.go`, so implement it after US1

### Within Each User Story

- Tests are written and fail before implementation
- `internal/pipeline` and `internal/executor` before `internal/engine`
- `internal/engine` before `cmd/truss`
- `go test` for the story's packages passes before the next story

### Parallel Opportunities

- T001 and T002 can run together
- T003 and T004 can run together
- T005–T009 can run together
- T010–T012 can run together after their tests exist
- T015–T017 can run together
- T022–T024 can run together
- T028 and T030 can run together

---

## Parallel Example: User Story 1

```bash
# Tests first, different files:
Task: "T005 parse contract test in internal/pipeline/parse_test.go"
Task: "T006 plan test in internal/pipeline/plan_test.go"
Task: "T007 host success test in internal/executor/host_test.go"
Task: "T008 engine success test in internal/engine/engine_test.go"
Task: "T009 CLI success contract test in cmd/truss/run_test.go"

# Then independent implementation:
Task: "T010 parse.go"
Task: "T011 plan.go"
Task: "T012 host.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: A valid `truss run pipeline.yaml` and a bare `truss run` of `truss.yaml` succeed
5. Demo the local run before adding rejection and fail-fast

### Incremental Delivery

1. Setup + Foundational
2. User Story 1 → successful local run (MVP)
3. User Story 2 → invalid definitions never execute
4. User Story 3 → fail-fast Result with the real Exit Code
5. Polish → `gofmt`, `go vet`, `go test ./...`, quickstart

### Parallel Team Strategy

After Foundational:

- One developer owns User Story 1 through the CLI
- User Story 2 and User Story 3 wait for those shared files, or take the parser/executor tests while User Story 1 is still red

---

## Notes

- [P] tasks use different files and do not depend on incomplete tasks
- [US1], [US2], and [US3] map to spec.md priorities P1, P2, and P3
- Confirm each test fails before writing the behavior it describes
- The `truss` process exits 0 or 1. The Step Exit Code stays in the report
- Each Step is a new shell in the invoker's original directory and environment
- Run `gofmt`, `go vet`, and `go test` for touched packages at each checkpoint
