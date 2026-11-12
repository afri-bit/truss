# Truss

## Project

TRUSS is a portable pipeline execution platform: a CLI, a local daemon with scheduling, and TRUSS Desktop.

The project is developed using Spec-Driven Development. Specifications define intended behavior before implementation decisions are made.

The core engine, CLI, and daemon are implemented in Go. SDKs and external integrations may use other languages and must not depend on Go-specific implementation details.

## Development Workflow

Behavior that users or integrations can depend on is written down in `specs/<feature>/spec.md`
and ships with tests, in the same pull request as the code. The order is yours: spec first is
the usual choice for a change to a public contract or to process execution; for other work the
spec may be written alongside or right after the code.

The full Spec Kit sequence (specify, clarify, plan, tasks, analyze, implement) is available
through `/feature` and the `/speckit-*` skills (see `CLAUDE.md`). Use as much of it as the
change needs.

Small mechanical changes such as formatting, typo fixes, dependency maintenance, or equivalent refactoring need no specification unless they alter observable behavior.

## Sources of Truth

Use the following project artifacts for their respective purposes:

* `docs/MILESTONE.md` defines what the current release includes and what waits until later. Read it before choosing or expanding a feature.
* `.specify/memory/constitution.md` defines project-wide engineering principles and constraints.
* `specs/<feature>/spec.md` defines required feature behavior and intent.
* `specs/<feature>/plan.md` and `tasks.md`, when present, define the technical approach and the work.
* `docs/architecture.md` maps the packages and their boundaries; `docs/decisions/` records why.
* Source code represents the current implementation, not automatically the intended behavior.

Do not duplicate feature requirements or architectural principles into this file.

If implementation and specification disagree, do not silently modify either side to make them match. Identify the discrepancy first.

## Before Implementing

Before modifying code for a feature:

1. Read this `AGENTS.md`.
2. Read `docs/MILESTONE.md` and stay inside the current release.
3. Read `.specify/memory/constitution.md`.
4. Identify the relevant feature under `specs/`.
5. Read its `spec.md`.
6. When present, read its `plan.md` and `tasks.md`.
7. Inspect the existing implementation relevant to the change.

Do not assume repository structure, APIs, or behavior from general conventions when the repository already defines them.

## Specification Discipline

Specifications describe what TRUSS must do and why.

Do not introduce implementation details into `spec.md` unless they are externally observable requirements or explicit constraints.

Technical decisions belong in `plan.md` when a feature has one, and in `docs/decisions/` when they outlive the feature.

Do not invent requirements that are not supported by the specification or explicit user direction.

When a requirement is materially ambiguous, surface it: ask, or decide and record the decision where a reviewer will see it.

During implementation, do not silently expand feature scope.

## Engineering Principles

Project-wide principles for code quality, testing, maintainability, Go practice, execution safety, and compatibility live in `.specify/memory/constitution.md`.

Do not restate, weaken, or silently diverge from those principles here. If this file and the constitution disagree, the constitution prevails; surface the conflict.

## Generated and Tooling Files

Treat `.specify/` as Spec Kit infrastructure and project SDD configuration.

Treat `.claude/` as the Claude Code agent setup: agents, skills, rules, hooks, and permissions. `CLAUDE.md` says how Claude Code works in this repository. `docs/agents.md` describes the whole setup.

Treat `.agent/` as shared agent memory, work state, and event log. Do not store requirements there.

Do not move project requirements into `.claude/` or `.agent/`.

Do not modify generated Spec Kit files or the agent setup unless the task specifically concerns that tooling.

The product architecture must not depend on Claude Code or Spec Kit being present at runtime.

## Completion

Before declaring implementation work complete:

1. Confirm the implemented behavior matches the active specification.
2. Confirm the implementation follows the technical plan, or document intentional deviations.
3. Confirm relevant tasks are completed.
4. Confirm constitution Quality Gates: `gofmt`, `go vet`, and relevant `go test` pass.
5. Check for unintended changes outside the feature scope.
6. Report unresolved limitations or deviations clearly.

Do not claim completion when required tests are failing or specified behavior remains unimplemented.
