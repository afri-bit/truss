---
description: "Task list for step concurrency (spec 004)"
---

# Tasks: Step Concurrency

**Input**: Design documents from `/specs/004-step-concurrency/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED (constitution Principle III). Inside every story the test tasks come first and MUST be run and seen to fail before the implementation tasks start. Keep the failing output.

**Organization**: Grouped by user story. Two pull requests (plan.md, Delivery): **PR-A** = Setup + Foundational (executor process trees, thread-safe run log, CI, helper). **PR-B** = US1 to US5 and Polish (schema, unknown fields, scheduler, CLI interrupt, docs).

## Format: `- [ ] ID [P?] [Story] Description — [low|high] [PR-A|PR-B] agent`

- **[P]**: different files, no dependency on an unfinished task
- **[low|high]**: model routing (`low` haiku, `high` sonnet). Process execution, secrets, contract and schema work is never `low`.
- Agents: `go-engineer`, `test-writer`, `devops-engineer`, `docs-writer`
- Rules: `.claude/rules/tdd.md`, `go-style.md`, `go-architecture.md`, `execution-safety.md`, `public-contracts.md`

## Path Conventions

Packages `cmd/truss`, `internal/{pipeline,engine,executor,runlog}`. Tests beside code. Test helper code is duplicated per package in `helper_test.go` on purpose: plan.md adds no package.

---

## Phase 1: Setup (shared infrastructure)

- [x] T001 [P] Update `.github/workflows/ci.yml`: run `gofmt -l .` (fail on output), `go vet ./...`, `golangci-lint run ./...`, `go test -race -count=1 ./...` on ubuntu, macos, windows; add a stress step `go test -race -count=20 -run 'Concurren|Stop|Tree' ./internal/engine ./internal/executor ./cmd/truss`. On Windows, if `gcc` is not on the runner, run the stress step without `-race` and print a notice. Baseline first: the local `golangci-lint` is built with Go 1.25 and cannot lint Go 1.27.3, and CI has never run it, so run it once in CI on the unchanged code; fix or record every finding before the step is made blocking. Pin no new action; keep existing ones — [high] [PR-A] devops-engineer
- [x] T002 [P] Repair `.github/scripts/e2e-test.sh` so it asserts the current run-log contract (labelled `step <id> |` lines, `finished ... exit N`, summary, `error: ` on stderr; remove the stale `[hello]` assertions), then add a CI step that runs it after `go build -o bin/truss ./cmd/truss` on all three OSes — [high] [PR-A] devops-engineer
- [x] T003 [P] Add `internal/executor/helper_test.go`: `TestMain` re-executes the test binary when env `TRUSS_TEST_HELPER` is set. Subcommands: `touch-and-wait <mine> <other>` (create `mine`, wait until `other` exists or a deadline passes, exit 0 or 3), `print-lines <n>`, `partial <text>` (write text with no newline, then exit), `spawn-grandchild <pidfile>` (start a child that writes its pid to `pidfile` and blocks, then block), `exit <code>`. Export a `helperCmd(args...) string` that returns a `run` string valid under `/bin/sh -c` and `cmd /c`. No sleeps as synchronization — [high] [PR-A] test-writer
- [x] T004 [P] Add the same helper in `internal/engine/helper_test.go` — [high] [PR-A] test-writer
- [x] T005 [P] Add the same helper in `cmd/truss/helper_test.go` (moved into PR-A: needed by the interrupt tests) — [high] [PR-A] test-writer

---

## Phase 2: Foundational (blocks all user stories) — PR-A

**Purpose**: give every Step a stoppable process tree and make the run log safe for concurrent writers. No schema or scheduling change yet; behavior at limit 1 stays spec 003 except FR-011 and FR-014.

**Independent test**: with the helper, `Execute` of a command that starts a blocking grandchild returns after the shell exits and the grandchild's pid is gone; cancelling the context ends the whole tree; many goroutines writing the run log produce only whole, labelled lines.

### Tests first

- [x] T006 [P] In `internal/executor/host_test.go` add failing tests: `Start`/`Wait` success returns `ExitCode` 0; a command that cannot start makes `Start` return an error and no process; `Execute` keeps its current contract (existing tests still pass); a non-zero exit keeps its Exit Code — [high] [PR-A] test-writer
- [x] T007 [P] In `internal/executor/tree_test.go` (shared) add failing tests named `TestStop_*`/`TestTree_*`: (a) cancelling the context ends the shell and the grandchild (pid not alive after `Wait`; Unix `syscall.Kill(pid,0)` returns `ESRCH`, Windows `OpenProcess` fails or `GetExitCodeProcess` is not `STILL_ACTIVE`; bounded poll); `Result.Stopped` is true; Unix `ExitCode` is nil with `Err` text `terminated by signal: killed`; Windows `ExitCode` is 1. (b) leftover: a command that starts a background grandchild and exits 0 — after `Wait` returns the grandchild is gone (FR-014). (c) a Step that exits 0 while a stop races is not `Stopped`. (d) a Step killed by an external signal has no Exit Code and a diagnostic, never `-1` (Unix; skip on Windows with a stated reason). Use `go test -count=20 -race` — [high] [PR-A] test-writer
- [x] T008 [P] In `internal/runlog/runlog_test.go` add failing tests (`-race`): several goroutines call `StepOutput` with partial chunks for different ids; every written line is complete with exactly one `step <id> |` label; each id's lines keep their order; timestamps never decrease; `StepStopped(id, by)` flushes the partial line then writes `step <id> stopped because step <by> failed` and `StepFinished` follows — [high] [PR-A] test-writer

### Implementation

- [x] T009 `internal/executor/host.go` and `result.go`: add `Start(ctx, req) (*Process, error)`, `(*Process).Wait(ctx) Result`, keep `Execute` as `Start`+`Wait`; use `exec.Command` (not `CommandContext`); `Result.Stopped bool` per data-model.md; a process with no host Exit Code yields `ExitCode == nil` and `Err` carrying the signal diagnostic (never `-1`); after the shell exits, end remaining processes of the tree and release it; wait for the tree to be gone before returning; widen the `Result.Err` doc comment. Run T006 and T007 until green on Linux — [high] [PR-A] go-engineer
- [x] T010 [P] `internal/executor/tree_unix.go` (`//go:build unix`): `SysProcAttr{Setpgid: true}`; `kill()` = `syscall.Kill(-pgid, SIGKILL)` ignoring `ESRCH`, idempotent; after shell exit kill the group again and poll `Kill(-pgid, 0)` until `ESRCH`, bounded to about 1 s; `release()` no-op — [high] [PR-A] go-engineer
- [x] T011 [P] `internal/executor/tree_windows.go` (`//go:build windows`): `CreationFlags |= CREATE_SUSPENDED (0x4)`; after `Start`, `CreateJobObjectW`, `AssignProcessToJobObject`, `NtResumeProcess` via `syscall.NewLazyDLL("kernel32.dll")` and `("ntdll.dll")`; `kill()` = `TerminateJobObject(job, 1)`, idempotent; `release()` closes the handle; if any job call fails, kill the suspended process and return a start error. No `golang.org/x/sys`, no `unsafe` unless unavoidable; do not widen `.golangci.yml` exclusions — [high] [PR-A] go-engineer
- [x] T012 `internal/runlog/runlog.go`: add `sync.Mutex`; every exported method holds it for the whole format, stamp and write; add `StepStopped(id, by string)` (flush the Step's partial line, then write the stopped line); keep `stamp()` monotonic. Run T008 with `-race -count=20` — [high] [PR-A] go-engineer
- [x] T013 [P] `CHANGELOG.md` `[Unreleased]`: add the two sequential-mode changes that land in PR A: leftover processes of a Step are ended when its shell exits; a Step ended by a signal logs a diagnostic instead of `exit -1` — [low] [PR-A] docs-writer

**Checkpoint (PR-A ready)**: `go vet`, `go test -race ./...` green on all three OSes in CI; stress step green. Open PR A, then continue.

---

## Phase 3: User Story 1 — Run independent steps at the same time (P1)

**Goal**: `pipeline.concurrency: N` lets up to N ready Steps run together, definition order, Dependencies honored.

**Independent test**: limit 2, two Steps that each create their own marker and wait for the other's marker; the run succeeds. With limit 1 or no limit the same file fails by its deadline.

### Tests first

- [ ] T014 [P] [US1] In `internal/pipeline/parse_test.go` and `plan_test.go` add failing tests: `concurrency: 2` parses to `Pipeline.Concurrency == 2`; omitted gives 0; `Plan` copies a value of at least 1 (0 becomes 1) — [high] [PR-B] test-writer
- [ ] T015 [P] [US1] In `internal/engine/engine_test.go` add failing tests `TestRun_Concurrent*` for spec scenarios 1–6: rendezvous pair succeeds at limit 2 and fails by deadline at limit 1; `a,b,c` at limit 2 (c starts only after a finish; assert via event order and a counter that never exceeds the limit); `build/lint/test(needs build)` at limit 3; `a,b(needs a),c,d(needs c)` at limit 2; limit larger than ready Steps; strict chain at limit 2 runs one at a time. Use a fake `StepEvents` that records order — [high] [PR-B] test-writer

### Implementation

- [ ] T016 [US1] `internal/pipeline/model.go`, `parse.go`, `plan.go`: add `Pipeline.Concurrency`, `ExecutionPlan.Concurrency`, `PipelineRun.FailedStep`, `StepRun.StoppedBy`; parse a valid decimal `!!int` of at least 1 under `pipeline.concurrency` (clamp to `math.MaxInt` on overflow); `Plan` copies a normalized value. Rejections come in US4 — [high] [PR-B] go-engineer
- [ ] T017 [US1] `internal/engine/engine.go`, `log.go`: implement the scheduler from research.md ("Scheduling"): one goroutine owns state; scan `p.Steps` in definition order; start synchronously while running < limit and every need succeeded; reserve the Step's slot in `run.Steps`; `Wait` in a goroutine reporting on a channel; call `StepStarted`/`StepOutput`/`StepFinished` as today; add `StepStopped(id, failedID string)` to `StepEvents`; keep `Plan(p)` validation. Make T015 pass; failure handling is US3 — [high] [PR-B] go-engineer

**Checkpoint**: US1 passes on its own with `go test -race -count=20 -run Concurren ./internal/engine`.

---

## Phase 4: User Story 2 — Read one log while steps run together (P2)

**Goal**: the log stays one readable text; shown equals saved.

**Independent test**: limit 2, two Steps printing several lines each; labelled complete lines, per-Step order, non-decreasing timestamps, saved log equals stdout.

### Tests first

- [ ] T018 [P] [US2] In `cmd/truss/run_test.go` add failing tests: overlapping Steps (helper `print-lines`) produce only complete lines each with one `step <id> |` label; each Step's lines are in order; timestamps never go backwards; `.truss/run.log` equals stdout; a partial line (`partial`) appears once, complete, at Step end — [high] [PR-B] test-writer
- [ ] T019 [P] [US2] In `cmd/truss/run_test.go` add failing tests: setup section and summary unchanged (status, Steps started, duration, finish time); started Steps listed in start order; no line shows the limit, any `needs`, or any environment variable name or value — [high] [PR-B] test-writer

### Implementation

- [ ] T020 [US2] Run T018 and T019 with `-race -count=20`. Fix gaps only in `internal/runlog/runlog.go` or `cmd/truss/run.go` (expected: none after T012 and T017). Record in the commit message if nothing needed changing — [high] [PR-B] go-engineer

---

## Phase 5: User Story 3 — Stop the whole run when a step fails (P3)

**Goal**: first failure fails the run, stops running Steps with their whole tree, starts nothing else; an interrupt does the same.

**Independent test**: limit 2; `a` exits 1, `b` runs and would write a marker, `c` would write another; run fails, no markers, `b` has a `stopped` line, `c` absent, no process left.

### Tests first

- [ ] T021 [P] [US3] In `internal/engine/engine_test.go` add failing tests `TestRun_Stop*`: failed `a` stops running `b` (event `StepStopped(b, a)` before `StepFinished(b)`); ready but unstarted `c` never starts and is absent from `run.Steps`; start failure while another Step runs (diagnostic, no Exit Code, other Step stopped); two near-simultaneous failures keep both Exit Codes and `FailedStep` is the first recorded; the grandchild pid of a stopped Step is gone after `Run` returns; a Step that exits 0 during the stop is `succeeded` — [high] [PR-B] test-writer
- [ ] T022 [P] [US3] In `cmd/truss/run_test.go` add failing tests: failed concurrent run → exit 1, summary failed, error line names the pipeline and `FailedStep`, stopped line placed right before the finish line, saved log equals stdout, no marker file; a Step killed by an external signal shows `error <diagnostic>` and no `exit` (Unix) — [high] [PR-B] test-writer
- [ ] T023 [P] [US3] In `cmd/truss/run_test.go` and `cmd/truss/main_test.go` add failing tests for FR-017: cancelling the context passed to `Run` while Steps run ends all Steps and their tree, prints `error: context canceled`, exits 1, leaves an existing `.truss/run.log` byte-for-byte unchanged, shows no `cancelled`; the context-cancel test runs on all three OSes; on Unix a second test builds the binary, sends SIGINT and SIGTERM, and checks the same; on Windows the console Ctrl-C is checked by hand and noted in the PR (SIGTERM does not exist there) — [high] [PR-B] test-writer

### Implementation

- [x] T024 [US3] **Done in PR-A after the Windows CI failure:** make Windows `endedByKill` precise — end the job with a distinctive exit code (not a constant true) and have `endedByKill` check it, so a Step that fails on its own is never labelled stopped; update research.md and contracts/execution.md (they say `exit 1`). `internal/engine/engine.go`: on the first failed outcome or `Start` error record `FailedStep`, cancel the run context, start no more Steps, emit `StepStopped` for Steps whose `Result.Stopped` is true, collect every Step's outcome before returning; return `ctx.Err()` when the parent context was cancelled. Make T021 pass — [high] [PR-B] go-engineer
- [ ] T025 [US3] `cmd/truss/run.go`: `failureLine` uses `run.FailedStep` instead of the last entry in `run.Steps`; `cmd/truss/main.go`: wrap the context with `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` (use a build-tag-free form that compiles on Windows). The signal handling and its tests moved into PR-A (review finding: Setpgid without it leaves Steps running after Ctrl-C). Make T022 and T023 pass — [high] [PR-B] go-engineer

**Checkpoint**: US1 to US3 pass together with `-race -count=20`.

---

## Phase 6: User Story 4 — Reject an invalid concurrency limit and unknown fields (P4)

**Goal**: invalid limits and unrecognised fields are rejected before any command starts.

**Independent test**: for each invalid value and each unknown field, with an existing `.truss/run.log`: error names the field, no command runs, log unchanged.

### Tests first

- [ ] T026 [P] [US4] In `internal/pipeline/parse_test.go` and `validate_test.go` add failing table tests: `0`, `-1`, `1.5`, `1.0`, `"two"`, `"3"`, `true`, `[]`, `{}`, empty, `null`, `0x10`, `0o7`, `1_000`, `+3` each rejected with `pipeline concurrency must be a whole number of at least 1, got "<value>"` (`got a list`/`got a mapping`); unknown field at top level, under `pipeline`, in a Step, and in a Step without an id (`in step N`) each rejected with the messages in contracts/pipeline-v1.md; check order (version, unknown field, name, concurrency, steps, id, run, needs); a valid limit with a bad graph gives the existing needs error; a definition with only recognised fields is accepted. **Rewrite** `parse_test.go` "depends_on is ignored" and `validate_test.go` `TestValidate_DependsOnIsNotNeeds` to expect the `unknown field "depends_on" in step "<id>"` rejection (FR-016 replaces the old rule; say so in the commit message) — [high] [PR-B] test-writer
- [ ] T027 [P] [US4] In `cmd/truss/run_test.go` add failing tests: each invalid value and a misspelled `concurency` run no command (marker absent), exit 1, print `error: ...`, and leave an existing `.truss/run.log` byte-for-byte unchanged. **Rewrite** `TestRun_DependsOnIsNotADependency` to expect rejection with exit 1 and no Step run — [high] [PR-B] test-writer

### Implementation

- [ ] T028 [US4] `internal/pipeline/parse.go`, `model.go`: extend the node walk with a `default:` branch at the three levels recording the first unknown key and its location; detect an invalid `concurrency` node (null, empty, wrong tag, non-decimal text, sequence, mapping) and keep its source text; do not fail `Parse` for these (schema checks stay in `Validate`) — [high] [PR-B] go-engineer
- [ ] T029 [US4] `internal/pipeline/validate.go`: report the unknown field and the invalid limit in the order and with the messages of contracts/pipeline-v1.md; also reject a negative `Concurrency` set from Go code; existing messages unchanged. Make T026 and T027 pass — [high] [PR-B] go-engineer

---

## Phase 7: User Story 5 — Leave pipelines without a limit unchanged (P5)

**Goal**: no limit or limit 1 behaves as spec 003 (apart from FR-011, FR-014, FR-016).

**Independent test**: run the spec 003 definitions with no limit and with `concurrency: 1`; order, fail-fast omission, log shape and exit codes match.

### Tests first

- [ ] T030 [P] [US5] In `internal/engine/engine_test.go` add tests: for every spec 003 table (`TestPlan`/engine order tables), at limit 1 the scheduler's start order equals `pipeline.Plan(p).Steps`; omitted and `1` give identical events — [high] [PR-B] test-writer
- [ ] T031 [P] [US5] In `cmd/truss/run_test.go` add tests: the spec 003 successful and failed definitions with no limit and with `concurrency: 1` produce the spec 002/003 log shape (timestamps stripped) and exit codes; a Step that starts a background process (`spawn-grandchild`) and exits 0 has that process ended when the Step finishes (FR-014); each Step still runs in the invoker's working directory with the invoker's environment and no command other than the Step `run` strings runs (FR-015) — [high] [PR-B] test-writer

### Implementation

- [ ] T032 [US5] Run T030 and T031. Fix any gap in `internal/engine/engine.go` or `cmd/truss/run.go`; if nothing fails, record that in the commit message. Do not weaken either test — [high] [PR-B] go-engineer

---

## Phase 8: Polish and cross-cutting (PR-B)

- [ ] T033 [P] Extend `.github/scripts/e2e-test.sh` with: a concurrent success (helper-built binary or `go run` of a tiny rendezvous program so it works under `sh` and Git Bash), a fail-fast stop with no marker, and an unknown-field rejection; run in CI on three OSes — [high] [PR-B] devops-engineer
- [ ] T034 [P] `README.md`: document `pipeline.concurrency`, `needs`, the recognised fields, the `stopped` line, interrupt, and repair the "Output" section to match the run-log contract — [low] [PR-B] docs-writer
- [ ] T035 [P] `CHANGELOG.md` `[Unreleased]`: add entries for specs 002 and 003 (missing today) and for 004 — Added (concurrency, `stopped` line, interrupt handling); Changed (unknown fields rejected, leftover processes ended at Step end, signal diagnostic instead of `-1`) — [low] [PR-B] docs-writer
- [ ] T036 [P] `docs/MILESTONE.md`: add spec 004 to the "Already specified" table and update the "next specification" paragraph — [low] [PR-B] docs-writer
- [ ] T037 `docs/decisions/0002-step-concurrency-and-process-trees.md` via `/adr`: optional `concurrency` in schema v1, strict fields, process-tree ownership per OS, stdlib-only Windows job object, Exit Code of a stopped Step, leftover cleanup at Step end, interrupt handling; status `Proposed` — [high] [PR-B] docs-writer
- [ ] T038 [P] `docs/GLOSSARY.md`: check "Concurrent Execution" and "Status"; edit only if wrong (expected: no change) — [low] [PR-B] docs-writer
- [ ] T039 Run `/verify` (gofmt, vet, golangci-lint, `go test -race ./...`, `-count=20` stress, e2e); then `/review` with `reviewer`, `spec-reviewer`, `security-reviewer`, `portability-reviewer`; fix blockers; update the work state and `/handoff` — [high] [PR-B] orchestrator
- [ ] T040 Tick finished tasks in this file, set `Status: Implemented` in `spec.md` and the checklist, list open tasks in the PR description — [low] [PR-B] docs-writer

---

## Dependencies and execution order

- Phase 1 → Phase 2 (PR-A). T003/T004 before T006–T008; T009 before T010/T011 can be merged (T010 and T011 are [P] with each other); T012 after T008; T013 any time in PR-A.
- Phase 2 blocks all stories. After PR-A: US1 (T014–T017) → US3 (T021–T025, needs the scheduler) → US2 (T018–T020, needs the scheduler) ; US4 (T026–T029) is independent of US2/US3 but US1's T016 and US4's T028 touch the same files, so do T016 first.
- US5 (T030–T032) after US1 and US3. Polish after all stories.
- Within each story: tests (fail first) → implementation. T005 before any `cmd/truss` helper test.

### Parallel opportunities

- T001, T002, T003, T004, T005 together. T006, T007, T008 together. T010 and T011 together.
- Within US1: T014 with T015. Within US2: T018 with T019. Within US3: T021, T022, T023. Within US4: T026 with T027. Within US5: T030 with T031. Polish: T033–T036, T038.
- Use `/fan-out` for the [P] groups only when they touch different files.

## Implementation strategy

1. PR-A: T001–T013. Merge before PR-B starts (or build PR-B on top).
2. PR-B: US1 (MVP: concurrent run works) → US3 (safe) → US2 → US4 → US5 → Polish.
3. Stop after each checkpoint and run `/verify`. Repeat `-count=20` for any test touching goroutines.
4. Needs a human: ADR 0002, the workflow and script changes under `.github/` (guardrail paths), and that the PRs share one session branch.
