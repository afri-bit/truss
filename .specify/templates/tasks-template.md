---
# Lifecycle: draft -> approved -> implementing -> done
# (see docs/development/spec-driven-workflow.md)
status: draft
feature: "[###-feature-name]"
document: tasks
description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: REQUIRED for behavioral changes. Each user story MUST include
test tasks that assert observable behavior. Integration or contract tests
are REQUIRED when the story changes process execution, CLI contracts, or
other public interfaces. Tests MUST be written so they can fail before
implementation exists.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description (FR-###)`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- **(FR-###)**: The acceptance criteria from spec.md this task satisfies;
  setup and polish tasks may use `(—)`
- Include exact file paths in descriptions
- Keep each task small enough for one reviewable pull request; the PR
  references the task ID and ticks its checkbox

## Path Conventions

- **TRUSS core (Go)**: packages under `cmd/`, `pkg/`, and `internal/` as
  described in docs/architecture.md. Tests are `*_test.go` beside the
  code; fixtures in `testdata/`; Docker-dependent tests carry the
  `integration` build tag.
- **SDKs / integrations**: language-idiomatic layout; public contracts
  MUST NOT depend on Go-specific types.
- Paths shown below assume the Go module layout — adjust from plan.md.

<!-- 
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.
  
  The /speckit.tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/
  
  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment
  
  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create Go module layout per implementation plan
- [ ] T002 Initialize Go module and justified dependencies
- [ ] T003 [P] Configure `gofmt` / `go vet` (and CI test invocation)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on your project):

- [ ] T004 Setup database schema and migrations framework
- [ ] T005 [P] Implement authentication/authorization framework
- [ ] T006 [P] Setup API routing and middleware structure
- [ ] T007 Create base models/entities that all stories depend on
- [ ] T008 Configure error handling and logging infrastructure
- [ ] T009 Setup environment configuration management

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1 (REQUIRED for behavioral changes)

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [ ] T010 [P] [US1] Unit tests for observable behavior in pkg/[pkg]/[name]_test.go (FR-001)
- [ ] T011 [P] [US1] Integration/contract test for [CLI or execution boundary] in pkg/[pkg]/[name]_test.go (FR-001)

### Implementation for User Story 1

- [ ] T012 [P] [US1] Define types in pkg/[pkg]/[name].go (FR-001)
- [ ] T013 [P] [US1] Define interfaces at the consumption boundary in pkg/[pkg]/ (FR-001)
- [ ] T014 [US1] Implement behavior in pkg/[pkg]/ (depends on T012, T013) (FR-001)
- [ ] T015 [US1] Wire CLI or daemon entry in cmd/[binary]/ only after core logic is independent of presentation (FR-001)
- [ ] T016 [US1] Propagate context, wrap errors, preserve exit codes (FR-001)
- [ ] T017 [US1] Ensure logs/diagnostics do not expose secrets (FR-001)

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2 (REQUIRED for behavioral changes)

- [ ] T018 [P] [US2] Unit tests for observable behavior in pkg/[pkg]/[name]_test.go (FR-001)
- [ ] T019 [P] [US2] Integration/contract test for [boundary] in pkg/[pkg]/[name]_test.go (FR-001)

### Implementation for User Story 2

- [ ] T020 [P] [US2] Define types in pkg/[pkg]/[name].go (FR-001)
- [ ] T021 [US2] Implement behavior in pkg/[pkg]/ (FR-001)
- [ ] T022 [US2] Wire entrypoint in cmd/[binary]/ if needed (FR-001)
- [ ] T023 [US2] Integrate with User Story 1 components (if needed) (FR-001)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3 (REQUIRED for behavioral changes)

- [ ] T024 [P] [US3] Unit tests for observable behavior in pkg/[pkg]/[name]_test.go (FR-001)
- [ ] T025 [P] [US3] Integration/contract test for [boundary] in pkg/[pkg]/[name]_test.go (FR-001)

### Implementation for User Story 3

- [ ] T026 [P] [US3] Define types in pkg/[pkg]/[name].go (FR-001)
- [ ] T027 [US3] Implement behavior in pkg/[pkg]/ (FR-001)
- [ ] T028 [US3] Wire entrypoint in cmd/[binary]/ if needed (FR-001)

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Documentation updates in docs/
- [ ] TXXX Confirm `gofmt` and `go vet` pass on changed packages
- [ ] TXXX Run `go test ./...` for affected modules
- [ ] TXXX [P] Review secret handling in logs, diagnostics, and artifacts
- [ ] TXXX Confirm public contracts remain versioned / compatibility noted
- [ ] TXXX Run quickstart.md validation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Types/packages before wiring
- Core execution before CLI/daemon presentation
- Story complete (including `go test` for touched packages) before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Types within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Unit tests for observable behavior in pkg/[pkg]/[name]_test.go"
Task: "Integration/contract test for [boundary] in pkg/[pkg]/[name]_test.go"

# Launch types for User Story 1 together:
Task: "Define types in pkg/[pkg]/[name].go"
Task: "Define interfaces at the consumption boundary in pkg/[pkg]/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Run `gofmt`, `go vet`, and `go test` for touched packages at story checkpoints
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
