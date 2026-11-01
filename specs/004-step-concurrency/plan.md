# Implementation Plan: Step Concurrency

**Branch**: `claude/truss-cursor-to-claude-18fzh9` (session-assigned; the constitution's name would be `feature/004-step-concurrency`) | **Date**: 2026-11-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-step-concurrency/spec.md`

## Summary

A Pipeline may declare `pipeline.concurrency`, a whole number of at least 1 that defaults to 1. The engine gets a small scheduler that starts ready Steps in definition order, never more than the limit at once, and starts the next ready Step each time one finishes. On the first observed failure it starts nothing more and force-ends every running Step's whole process tree. Each Step becomes the root of its own process tree (a process group on Linux and macOS, a job object on Windows), built with the standard library only. The run log becomes goroutine-safe and gains one line, `step <id> stopped because step <other> failed`. Unknown fields are rejected at every level. An interrupt (Ctrl-C or SIGTERM) ends all Steps and exits 1 without saving a log. At limit 1 the scheduler reduces exactly to spec 003. The work ships as two pull requests: A (executor process trees and the thread-safe run log) and B (schema, scheduler, CLI, docs).

## Technical Context

**Language/Version**: Go 1.27.3 (module `github.com/afri-bit/truss`)
**Primary Dependencies**: Go standard library; `gopkg.in/yaml.v3` for YAML decoding only. No new module: Windows job objects use `syscall.NewLazyDLL`, not `golang.org/x/sys`.
**Storage**: N/A (`.truss/run.log` as in spec 002)
**Testing**: `go test -race ./...`; contract tests for schema validation, scheduling order, fail-fast, the run log, and the CLI; process-tree tests with a re-exec helper process; test-first per constitution Principle III
**Target Platform**: Linux, macOS, Windows (supported hosts in `docs/MILESTONE.md`). Validation, ordering, and log shape do not vary by host; process stopping does and is specified per host in [research.md](./research.md).
**Project Type**: CLI + engine (daemon is out of scope)
**Performance Goals**: N/A. Correctness under concurrency and complete cleanup matter, not throughput.
**Constraints**: Only Step `run` strings are executed. Exit Codes are preserved; a missing Exit Code is a diagnostic, never `-1`. A failed Step fails the run; running Steps are stopped immediately and forcefully with their whole process tree; nothing started by a Step outlives `truss` or its Step. Secrets and environment never reach the log. `context.Context` stays the first parameter of blocking calls and is not stored on structs. No new module.
**Scale/Scope**: One pipeline file, one local run, tens of Steps

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
- [x] Agent work follows the constitution's Agentic Operation: independent review, guardrail changes listed for the maintainer, decisions logged
- [x] Complexity Tracking records any principle violation and the simpler alternative rejected

Post-design re-check (2026-11-01): all gates pass. The engine gains a scheduler in `internal/engine/engine.go` and does not import `runlog` or the CLI (it keeps the consumer-owned `StepEvents` interface). Process-tree handling lives behind `executor.Start`/`Wait` in `internal/executor` with `tree_unix.go` and `tree_windows.go`; no package is added. `internal/pipeline` still runs nothing. `internal/runlog` only formats. The CLI owns signal handling and exit codes. Compatibility is recorded in [contracts/](./contracts/): the optional `pipeline.concurrency` field, strict unknown fields, the `stopped` log line, leftover process cleanup, and the signal diagnostic. Schema version stays `"1"`; no release has been tagged. An ADR is required (`docs/decisions/0002-step-concurrency-and-process-trees.md`) because this changes process execution and two public contracts. The constitution requires a maintainer decision for a new third-party module, so the plan avoids one.

## Project Structure

### Documentation (this feature)

```text
specs/004-step-concurrency/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── pipeline-v1.md
│   ├── execution.md
│   ├── run-log.md
│   └── cli.md
└── tasks.md              # created by /speckit-tasks, not this command
```

### Source Code (repository root)

```text
cmd/truss/
├── main.go              # signal-aware context (FR-017)
├── run.go               # failureLine uses PipelineRun.FailedStep
└── run_test.go
internal/
├── pipeline/            # model, parse (strict fields, concurrency), validate, plan
├── engine/              # scheduler, StepEvents.StepStopped
├── executor/
│   ├── host.go          # Start, Wait, Execute, Result.Stopped
│   ├── tree_unix.go     # //go:build unix: process group
│   ├── tree_windows.go  # //go:build windows: job object
│   └── result.go
└── runlog/              # mutex, StepStopped
.github/
├── workflows/ci.yml     # -race, stress runs, e2e, lint on three OSes
└── scripts/e2e-test.sh  # stale assertions repaired, new cases
docs/decisions/0002-step-concurrency-and-process-trees.md
```

**Structure Decision**: Keep the existing five packages. The scheduler is a function in `internal/engine` because it is the only consumer of "which Step next". The process-tree code is per-OS files in `internal/executor` because the operating system owns that boundary. Tests live beside the code; helper-process tests use a re-exec of the test binary instead of external tools so they behave the same under `sh` and `cmd.exe`.

## Delivery

Two pull requests, each linking this folder:

- **PR A** (foundation; no schema change, but two sequential-mode behavior changes land here, FR-011 and FR-014, and are recorded in `CHANGELOG.md` in the same PR): process trees in the executor, `Result.Stopped`, signal-death diagnostic and leftover cleanup, the mutex and `StepStopped` in the run log, CI additions, and the test helper.
- **PR B**: `pipeline.concurrency`, strict unknown fields, the scheduler, `FailedStep`, CLI interrupt handling, README, CHANGELOG, MILESTONE, and the ADR.

Both PRs together exceed the roughly 400-line guideline; the split keeps each reviewable. This session can push to one branch only, so they are built in that order on it.

## Known limitations and follow-ups

- Waiting for a Step's tree to disappear is bounded (about one second on Unix). If a process is still there after the bound, `Wait` returns anyway, logs nothing extra, and the run continues; this covers the same escapees as the `setsid` limitation below.

- A process that detaches from the Step's process group or job (`setsid` on Unix) can survive the stop. Documented, not preventable with the standard library.
- Duplicate YAML keys (for example `concurrency` twice) are not detected; the last wins. Follow-up.
- Windows: a force-killed `truss` leaves its jobs open; `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` needs a struct and `unsafe`. Follow-up.
- `.github/scripts/e2e-test.sh` and the README "Output" section are stale against the run-log contract; CI runs neither e2e, `-race`, nor `golangci-lint`. PR A repairs the script and CI; README is repaired in PR B.
- `CHANGELOG.md` `[Unreleased]` has no entries for specs 002 and 003; PR B adds them with 004.

## Complexity Tracking

No principle violation. The per-OS `tree_*.go` files and the `Start`/`Wait` split are the minimum that gives each Step a stoppable process tree; the simpler alternative (`exec.CommandContext`, which kills only the shell) cannot satisfy FR-014.
