# Tasks: Step Dependencies

**Input**: Design documents from `/specs/003-step-dependencies/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED. Write each story's tests first and confirm they fail before that story's implementation. Contract tests cover `contracts/pipeline-v1.md`, `contracts/execution.md`, and `contracts/run-log.md`.

**Organization**: Tasks are grouped by user story so each story can be implemented, tested, and demonstrated on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label ([US1], [US2], [US3], [US4])
- Every task includes a file path

## Path Conventions

- Go module `github.com/afri-bit/truss`
- `internal/pipeline/` parse, validate, and sequential plan
- `internal/engine/` existing fail-fast walk of the plan
- `cmd/truss/` existing CLI; contract tests in `run_test.go`
- `internal/runlog/` unchanged line format
- Tests in `*_test.go` beside the code

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: State the package boundary from plan.md. No new package and no new dependency.

- [ ] T001 Update `internal/pipeline/doc.go` so the package comment says it parses optional `needs`, validates the dependency graph, plans sequential order, and does not execute steps

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `needs` is present on the parsed model. Validation and planning stay unchanged in this phase.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Tests (REQUIRED)

> Write this test first and confirm it fails before T003.

- [ ] T002 Add parse tests in `internal/pipeline/parse_test.go`: omitted `needs` and `needs: []` produce no dependencies; `needs: [build]` and a trimmed entry `" build "` produce `build`; a scalar `needs` and a non-string entry do not fail `Parse` and are marked with the same unexported shape flag pattern as a non-string `run`; `depends_on` and any other unknown field leave `Needs` empty

### Implementation

- [ ] T003 Parse step `needs` in `internal/pipeline/parse.go` and store it on `Step` in `internal/pipeline/model.go`. Require `!!str` entries, trim them like `id`, and ignore `depends_on`. Do not change `Validate` or `Plan` in this task

**Checkpoint**: A parsed Step carries its declared `Needs`. User stories can begin.

---

## Phase 3: User Story 1 - Run steps after the steps they need (Priority: P1) 🎯 MVP

**Goal**: A valid definition runs one Step at a time. A Step starts only after every Step it needs has succeeded. When several Steps are ready, the one written earlier runs next. The run log shows that order and does not list `needs`.

**Independent Test**: Run a definition whose first Step needs a Step written later, and whose commands succeed. The needed Step runs first. The log shows that order with timestamps, durations, and Exit Codes, and it has no `needs` line.

### Tests for User Story 1 (REQUIRED)

> Write these tests first and confirm they fail before T007.

- [ ] T004 [P] [US1] Add plan tests in `internal/pipeline/plan_test.go` per `specs/003-step-dependencies/contracts/execution.md` for the orders `build` then `test`; `lint` then `test` then `package`; `build` then `report` then `scan`; and `lint` then `package` then `publish`. Assert each planned Step still carries only its declared `Needs`
- [ ] T005 [P] [US1] Add an engine test in `internal/engine/engine_test.go` that a Step written first and needing `build` does not run until `build` succeeds, and that `scan` with no `needs` runs after that dependent when `scan` is written last
- [ ] T006 [P] [US1] Add a CLI contract test in `cmd/truss/run_test.go` for `test` needs `build`, then `build`, then `scan`: exit 0, start order `build`, `test`, `scan`, the local run log setup and `summary succeeded steps 3`, and no line that lists `needs`

### Implementation for User Story 1

- [ ] T007 [US1] Implement the selection rule in `internal/pipeline/plan.go`: repeatedly choose the not-yet-chosen Step with the smallest definition index whose `Needs` are already chosen. Return an error if a Step remains and none is eligible. Leave `internal/engine/engine.go` as a walk of `ExecutionPlan.Steps`

**Checkpoint**: User Story 1 passes on its own. A valid graph runs in dependency order.

---

## Phase 4: User Story 2 - Reject an invalid dependency graph (Priority: P2)

**Goal**: An unknown id, a self-dependency, a cycle, a repeated id, or a `needs` value that is not a list of Step ids is rejected before any command starts. An existing `.truss/run.log` stays unchanged.

**Independent Test**: For each invalid case, run that definition while `.truss/run.log` exists. The error names the offending Step, no Step command runs, and the stored log is unchanged.

### Tests for User Story 2 (REQUIRED)

> Write these tests first and confirm they fail before T010.

- [ ] T008 [P] [US2] Add validation tests in `internal/pipeline/validate_test.go` per `specs/003-step-dependencies/contracts/pipeline-v1.md` and `specs/003-step-dependencies/data-model.md`: `step "<id>" needs must be a list of step ids`; `step "<id>" needs unknown step "<entry>"` including an empty entry; `step "<id>" depends on itself`; `step "<id>" repeats step "<entry>"`; `dependency cycle: a, b` when `a` is written first and each needs the other; `dependency cycle: a, b, c` when `a` needs `b`, `b` needs `c`, and `c` needs `a`
- [ ] T009 [P] [US2] Add CLI contract tests in `cmd/truss/run_test.go` for those five faults: `error: <diagnostic>` on standard error, exit 1, no step line, and an existing `.truss/run.log` byte-for-byte unchanged

### Implementation for User Story 2

- [ ] T010 [US2] Implement the `needs` checks in `internal/pipeline/validate.go` after the current structural checks, one first fault in definition order, and inside a list in entry order: shape, unknown id, self, duplicate, then one cycle named from the member written earliest and then following `needs`. Keep the existing `error:` path in `cmd/truss/run.go`

**Checkpoint**: User Stories 1 and 2 both work. Illegal graphs run nothing.

---

## Phase 5: User Story 3 - Stop the whole run when a step fails (Priority: P3)

**Goal**: A failed Step fails the Pipeline Run and starts nothing further. Steps that need it, and Steps that were ready and do not need it, are omitted from the saved run log.

**Independent Test**: Run a definition where `test` is written before `build`, `test` needs `build`, `build` exits non-zero, and `lint` needs nothing. Only `build` runs. Its Exit Code is in the saved log. `test` and `lint` are absent.

### Tests for User Story 3 (REQUIRED)

> Write these tests first and confirm they fail before T013. The case below fails while `Plan` still follows definition order, because `test` would run before `build`.

- [ ] T011 [P] [US3] Add an engine test in `internal/engine/engine_test.go`: `test` needs `build` and is written first, `build` exits non-zero, `lint` has no `needs` and is last. Assert only `build` is in the result, with that Exit Code. Add a second case where the selected Step cannot start: that Step has a diagnostic and no Exit Code, and no later Step is present
- [ ] T012 [P] [US3] Add a CLI contract test in `cmd/truss/run_test.go` per `specs/003-step-dependencies/quickstart.md` section 4 and `specs/003-step-dependencies/contracts/run-log.md`: `build` exits non-zero, `test` needs `build`, `lint` does not. Expect exit 1, `build` with its exit code, `summary failed steps 1`, an error line naming the pipeline and `build`, no `test` or `lint`, and `.truss/run.log` matching that text

### Implementation for User Story 3

- [ ] T013 [US3] In `internal/engine/engine.go`, return on the first failed Step so later plan Steps do not start, including a Step that does not need the failed Step. Keep the failed Step's Exit Code, or its start error and a nil Exit Code, on the result

**Checkpoint**: A failure stops the whole run. The saved log contains only the Steps that started.

---

## Phase 6: User Story 4 - Leave definitions without dependencies unchanged (Priority: P4)

**Goal**: A definition that does not use `needs` still runs in definition order, with the same fail-fast behavior, run log, load rules, and process exit codes as today. `depends_on` does not create a Dependency.

**Independent Test**: Run a successful two-Step definition and a three-Step definition whose second Step fails, neither containing `needs`. Order, omission of the third Step, saved log, and exit codes match the local run log. A file that uses `depends_on` and not `needs` still runs in definition order.

### Tests for User Story 4 (REQUIRED)

> Write these tests first. They pass against today's definition-order behavior. Confirm they still pass after T017, and fix the implementation if any fail.

- [ ] T014 [P] [US4] Add plan tests in `internal/pipeline/plan_test.go`: Steps with `needs` omitted, and Steps with `needs: []`, stay in definition order
- [ ] T015 [P] [US4] Add validation tests in `internal/pipeline/validate_test.go`: a missing `run` is still reported when a later Step has a bad `needs`; `depends_on` alone does not produce a `needs` error; the existing missing-name, empty-steps, duplicate-id, empty-run, and bad-version messages stay
- [ ] T016 [P] [US4] Add CLI contract tests in `cmd/truss/run_test.go`: a definition without `needs` runs in definition order and exits 0 with the log saved; a second Step exiting non-zero omits the third Step and exits 1; `truss run` with no path loads `truss.yaml`; `depends_on` without `needs` does not change definition order

### Implementation for User Story 4

- [ ] T017 [US4] Keep omitted and empty `needs` in definition order in `internal/pipeline/plan.go`, keep structural errors ahead of `needs` errors in `internal/pipeline/validate.go`, and keep `depends_on` ignored in `internal/pipeline/parse.go`

**Checkpoint**: All four stories pass. Existing definitions behave as they do today.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Quality gates that apply to the whole feature

- [ ] T018 [P] Walk through `specs/003-step-dependencies/quickstart.md` against the built `truss` CLI
- [ ] T019 Run `gofmt`, `go vet`, and `go test ./...` from the module root that contains `go.mod`
- [ ] T020 [P] Review `internal/pipeline/validate.go` and `cmd/truss/run.go` so `needs` diagnostics name Step ids only and do not add environment variable names or values
- [ ] T021 Confirm `specs/003-step-dependencies/contracts/pipeline-v1.md` and `specs/003-step-dependencies/contracts/run-log.md` still record that schema version `"1"` stayed and that the run log gained no `needs` line

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup. Blocks all user stories
- **User Stories (Phases 3–6)**: Depend on Foundational
- **Polish (Phase 7)**: Depends on the user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational. No dependency on US2, US3, or US4
- **User Story 2 (P2)**: Starts after Foundational. `internal/pipeline/validate.go` does not depend on US1. `cmd/truss/run_test.go` is shared with US1, so T009 follows T006
- **User Story 3 (P3)**: Starts after Foundational. T011 and T012 go green only after T007, because the failing Step must be selected before the Step that needs it. T013 follows those tests
- **User Story 4 (P4)**: Starts after Foundational. T014 follows T004, T015 follows T008, and T016 follows T006, T009, and T012, because they extend the same test files

### Within Each User Story

- Tests are written and fail before implementation, except the US4 regression tests, which must pass before and after T017
- `internal/pipeline` before any CLI change. This feature does not require a new branch in `cmd/truss/run.go`
- `go test` for the story's packages passes before the next story

### Parallel Opportunities

- T004, T005, and T006 can run together
- T008 can run with US1. T009 waits for T006
- T011 and T012 can run together after T007
- T014, T015, and T016 can run together once their test files are free
- T018 and T020 can run together

---

## Parallel Example: User Story 1

```bash
# Tests first, different files:
Task: "T004 plan order tests in internal/pipeline/plan_test.go"
Task: "T005 engine order test in internal/engine/engine_test.go"
Task: "T006 CLI order test in cmd/truss/run_test.go"

# Then:
Task: "T007 selection rule in internal/pipeline/plan.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: A definition that needs a later Step runs that Step first, and the run log shows that order
5. Demo the ordered run before rejection and fail-fast cases

### Incremental Delivery

1. Setup + Foundational → `needs` is parsed
2. User Story 1 → valid graphs run in order (MVP)
3. User Story 2 → illegal graphs run nothing and leave `.truss/run.log` unchanged
4. User Story 3 → a failed Step starts nothing further
5. User Story 4 → definitions without `needs` stay on today's behavior
6. Polish → `gofmt`, `go vet`, `go test ./...`, quickstart

### Parallel Team Strategy

After Foundational:

- One developer owns User Story 1 in `internal/pipeline/plan.go`
- Another owns User Story 2 in `internal/pipeline/validate.go`
- User Story 3 and User Story 4 wait for the shared `*_test.go` files, or write the engine test while User Story 1 is still red

---

## Notes

- [P] tasks use different files and do not depend on incomplete tasks
- [US1], [US2], [US3], and [US4] map to spec.md priorities P1, P2, P3, and P4
- Confirm each new behavior test fails before writing the behavior it describes
- The `truss` process exits 0 or 1. The step Exit Code stays in the run log
- Schema version stays `"1"`. The run log does not list `needs`
- `depends_on` stays an ignored field
- Run `gofmt`, `go vet`, and `go test` for touched packages at each checkpoint
