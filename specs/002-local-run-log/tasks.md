# Tasks: Local Run Log

**Input**: Design documents from `/specs/002-local-run-log/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED. Write each story's tests first and confirm they fail before that story's implementation. Contract tests cover the run log text, `.truss/run.log`, and the CLI commands in `contracts/run-log.md` and `contracts/cli.md`.

**Organization**: Tasks are grouped by user story so each story can be implemented, tested, and demonstrated on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label ([US1], [US2], [US3], [US4])
- Every task includes a file path

## Path Conventions

- Go module `github.com/afri-bit/truss`
- `cmd/truss/` CLI, help, version, and `.truss/run.log`
- `internal/runlog/` timestamped transcript text
- `internal/engine/` step events, no transcript formatting
- `internal/executor/` host shell path shared with the setup line
- Tests in `*_test.go` beside the code

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package boundary from plan.md

- [X] T001 Add package docs stating that `internal/runlog` formats the run log and does not parse YAML or execute commands, in `internal/runlog/doc.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The shell fact and the step-event boundary every story uses. No transcript text and no `.truss` file in this phase.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T002 [P] Add a host test that the reported shell is `/bin/sh` on Unix and `%COMSPEC%` or `cmd.exe` on Windows in `internal/executor/host_test.go`
- [X] T003 Expose that shell path from `internal/executor/host.go` and use it when starting a step, so the setup line and execution cannot disagree
- [X] T004 [P] Define `StepStarted`, `StepOutput`, and `StepFinished` on a step-event interface in `internal/engine/log.go` without changing `Run` yet

**Checkpoint**: The shell path and the event interface exist. User stories can begin.

---

## Phase 3: User Story 1 - Read a run as it happens (Priority: P1) 🎯 MVP

**Goal**: A successful `truss run` prints the setup section, each step with timestamps, output, duration, and Exit Code, and a succeeded summary.

**Independent Test**: Run a valid two-step pipeline whose commands succeed. The setup section appears before any step output, both steps appear in definition order with start, output, finish, duration, and Exit Code 0, every TRUSS line has a local timestamp, and the summary reports success, 2 steps, and the run duration.

### Tests for User Story 1 (REQUIRED)

> Write these tests first and confirm they fail before T008–T010.

- [X] T005 [P] [US1] Add contract tests for the success transcript in `internal/runlog/runlog_test.go` per `specs/002-local-run-log/contracts/run-log.md`: timestamp `YYYY-MM-DD HH:MM:SS`, a clock step backward does not move the timestamp earlier, duration uses three decimal seconds, five setup lines and no environment variable names, a step with one output line, a step with empty output, and `summary succeeded steps 2`
- [X] T006 [P] [US1] Add an engine test that two successful steps emit started, output, and finished events in definition order and that the engine writes no transcript text, in `internal/engine/engine_test.go`
- [X] T007 [P] [US1] Update the success CLI contract tests in `cmd/truss/run_test.go` to expect the run-log transcript on standard output for `truss run <path>`, including setup before step output and `summary succeeded steps 2`, and confirm they fail against the current `[id]` transcript

### Implementation for User Story 1

- [X] T008 [US1] Implement the run log writer in `internal/runlog/runlog.go`: injected clock, monotonic timestamps, setup, step start, one occurrence per output line, step finish with duration and exit, and the succeeded summary
- [X] T009 [US1] Emit step events from `internal/engine/engine.go`, stop writing `[id]` lines there, keep validation, definition order, fail-fast, and the structured result, and skip events when the sink is nil
- [X] T010 [US1] After a valid definition, write setup and the succeeded summary through `internal/runlog` to standard output from `cmd/truss/run.go`, and pass that writer to the engine. Do not create `.truss/run.log` in this task

**Checkpoint**: User Story 1 passes on its own. A successful local run prints the new transcript.

---

## Phase 4: User Story 2 - Find the latest run afterward (Priority: P2)

**Goal**: The finished run log is saved at `.truss/run.log` in the working directory and replaced by the next finished run.

**Independent Test**: Complete a run and confirm `.truss/run.log` matches the text just shown. Complete a second run and confirm the file is only the second run. Run a definition that lives outside the working directory and confirm the log is still in the working directory.

### Tests for User Story 2 (REQUIRED)

> Write these tests first and confirm they fail before T012.

- [X] T011 [P] [US2] Add CLI tests in `cmd/truss/run_test.go` that a finished run's `.truss/run.log` equals standard output, a second finished run replaces that file, and `truss run` of a file outside the working directory still writes `.truss/run.log` in the working directory

### Implementation for User Story 2

- [X] T012 [US2] In `cmd/truss/run.go`, after validation succeeds, write the same run-log bytes to standard output and `.truss/run.log.tmp`, then rename that temporary file to `.truss/run.log` when the run finishes. Do not edit ignore files

**Checkpoint**: User Stories 1 and 2 both pass. The latest run can be opened after the terminal scrolls away.

---

## Phase 5: User Story 3 - Keep a record only when a run starts (Priority: P3)

**Goal**: A rejected definition changes nothing. A failed run saves the log through the failed step and omits later steps. An unwritable `.truss` starts no step.

**Independent Test**: With `.truss/run.log` already present, run an invalid definition and confirm the error, no step, and an unchanged file. Then run a three-step pipeline whose second command exits non-zero and confirm the saved log has the first step, the failed second step with its Exit Code and duration, and no third step.

### Tests for User Story 3 (REQUIRED)

> Write these tests first and confirm they fail before T015.

- [X] T013 [P] [US3] Add run-log tests in `internal/runlog/runlog_test.go` for a non-zero `finished` line, a start failure with duration and no exit code, `summary failed`, and `error <pipeline> failed at <id>` per `specs/002-local-run-log/contracts/run-log.md`
- [X] T014 [P] [US3] Add CLI tests in `cmd/truss/run_test.go`: invalid YAML and a validation error leave an existing `.truss/run.log` unchanged and print no step line; a three-step run whose second command fails saves that log, omits the third step, and exits 1; when `.truss` is a file, the command prints an error, starts no step, and does not replace a log

### Implementation for User Story 3

- [X] T015 [US3] Write the failed summary and error line from `internal/runlog/runlog.go`, save that log from `cmd/truss/run.go` when the pipeline run fails, and return a user-visible error before any step when `.truss` or `.truss/run.log.tmp` cannot be created

**Checkpoint**: User Stories 1, 2, and 3 pass. A rejected definition cannot be mistaken for a failed run.

---

## Phase 6: User Story 4 - Ask for help or the version (Priority: P4)

**Goal**: Help and version answer without starting a pipeline. Any other invocation prints usage and exits non-zero.

**Independent Test**: With no pipeline file, `truss help` and `truss version` exit 0 and do not create `.truss/run.log`. An unknown command prints usage and exits 1.

### Tests for User Story 4 (REQUIRED)

> Write these tests first and confirm they fail before T017.

- [X] T016 [P] [US4] Add CLI contract tests in `cmd/truss/run_test.go` per `specs/002-local-run-log/contracts/cli.md`: `truss`, `truss help`, and `truss --help` print usage naming `run`, `help`, and `version` and exit 0 without creating `.truss/run.log`; `truss version` and `truss --version` print the `// version:` value from `go.mod` and exit 0; any other invocation prints usage to standard error and exits 1; `truss run --help` is a path

### Implementation for User Story 4

- [X] T017 [US4] Implement usage text, command dispatch, and the version constant that matches `go.mod` in `cmd/truss/main.go`, `cmd/truss/run.go`, and `cmd/truss/version.go`

**Checkpoint**: All four stories pass independently.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates that apply to the whole feature

- [X] T018 [P] Walk through `specs/002-local-run-log/quickstart.md` against the built `truss` CLI
- [X] T019 Run `gofmt`, `go vet`, and `go test ./...` from the module root that contains `go.mod`
- [X] T020 [P] Review `internal/runlog/runlog.go` and `cmd/truss/run.go` so the setup section and summary contain no environment variable names or values
- [X] T021 Confirm `specs/002-local-run-log/contracts/cli.md` still records that the unreleased host-engine transcript changed and schema version `"1"` did not

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup. Blocks all user stories
- **User Stories (Phases 3–6)**: Depend on Foundational
- **Polish (Phase 7)**: Depends on the user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational. No dependency on US2, US3, or US4
- **User Story 2 (P2)**: Starts after Foundational. Extends `cmd/truss/run.go` from US1, so implement it after US1
- **User Story 3 (P3)**: Starts after Foundational. Extends `internal/runlog/runlog.go` and `cmd/truss/run.go`, so implement it after US1. The saved failure log also uses the US2 file replace
- **User Story 4 (P4)**: Starts after Foundational. Extends `cmd/truss/run.go` and `cmd/truss/main.go`, so implement it after US1 unless those files are split first

### Within Each User Story

- Tests are written and fail before implementation
- `internal/runlog` and `internal/engine` before `cmd/truss`
- `go test` for the story's packages passes before the next story

### Parallel Opportunities

- T002 and T004 can run together
- T005, T006, and T007 can run together
- T013 and T014 can run together
- T018 and T020 can run together

---

## Parallel Example: User Story 1

```bash
# Tests first, different files:
Task: "T005 run log contract test in internal/runlog/runlog_test.go"
Task: "T006 engine step-event test in internal/engine/engine_test.go"
Task: "T007 CLI success transcript test in cmd/truss/run_test.go"

# Then in order:
Task: "T008 runlog.go"
Task: "T009 engine.go"
Task: "T010 run.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: A successful `truss run` prints setup, both steps, durations, Exit Codes, and the succeeded summary
5. Demo the live log before saving it or adding help

### Incremental Delivery

1. Setup + Foundational
2. User Story 1 → live success transcript (MVP)
3. User Story 2 → `.truss/run.log` matches that transcript
4. User Story 3 → rejection leaves the file unchanged, and a failed run saves the failure
5. User Story 4 → help and version
6. Polish → `gofmt`, `go vet`, `go test ./...`, quickstart

### Parallel Team Strategy

After Foundational:

- One developer owns User Story 1 through the CLI
- User Story 2, User Story 3, and User Story 4 wait for `cmd/truss/run.go`, or take the `internal/runlog` and `internal/engine` tests while User Story 1 is still red

---

## Notes

- [P] tasks use different files and do not depend on incomplete tasks
- [US1], [US2], [US3], and [US4] map to spec.md priorities P1, P2, P3, and P4
- Confirm each test fails before writing the behavior it describes
- The `truss` process exits 0 or 1. The step Exit Code stays in the run log
- Timestamps are local `YYYY-MM-DD HH:MM:SS` and do not move backward. Duration stays beside them
- `.truss/run.log` is replaced only after a run starts and finishes. This feature does not edit ignore files
- Run `gofmt`, `go vet`, and `go test` for touched packages at each checkpoint
