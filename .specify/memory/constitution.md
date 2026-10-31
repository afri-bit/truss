<!--
Sync Impact Report
- Version change: 1.2.0 → 1.3.0
- Modified principles: V (runtime independence now names Claude Code, not Cursor)
- Added sections: Agentic Operation (agent tooling, autonomy limits, shared memory)
- Removed sections: Branching tooling note (Spec Kit 1.2.x no longer infers the
  spec folder from the branch name, so the prefixed-branch workaround is obsolete)
- Templates requiring updates:
  - .specify/templates/plan-template.md ⚠ pending: `[###-feature-name]`
    branch placeholder has no prefix (Spec Kit tooling; change only in a
    task that concerns that tooling)
  - .specify/templates/spec-template.md ⚠ pending: same branch placeholder
  - .specify/templates/tasks-template.md ✅ no branch references
  - AGENTS.md ✅ updated: tooling section names `.claude/`, workflow points to CLAUDE.md
  - CLAUDE.md, docs/agents.md ✅ added: orchestration, risk classes, autonomy
  - .cursor/ ✅ removed; the Spec Kit integration is now `claude`
- Follow-up TODOs: none
-->


# TRUSS Constitution

## Core Principles

### I. Specification-Governed Behavior

TRUSS is developed with Spec-Driven Development. Required behavior MUST
be defined in `specs/<feature>/spec.md` before implementation of new
capabilities or behavioral changes. Technical decisions MUST live in
`plan.md`. Implementation work MUST live in `tasks.md`.

Source code records the current implementation, not automatically the
intended behavior. When implementation and specification disagree, the
discrepancy MUST be identified before either side is changed. Agents and
contributors MUST NOT invent requirements, silently expand scope, or
resolve material ambiguity by arbitrary choice.

Mechanical changes (formatting, typo fixes, dependency maintenance, or
equivalent refactoring) MAY proceed without a new specification only when
observable behavior is unchanged.

**Rationale**: Portable pipeline behavior must be reviewable and testable
independently of any one implementation language or contributor.

### II. Idiomatic Go Quality

The core engine, CLI, and daemon MUST be implemented in idiomatic Go.

Packages MUST be focused and cohesive around one purpose. Exported APIs
MUST stay minimal until they are intentionally part of a supported public
contract. Concrete types MUST be preferred over premature abstraction.
Interfaces MUST be introduced at meaningful behavioral boundaries
(multiple implementations, isolation, or testing) and accepted where they
are consumed, not invented as broad hierarchies in advance.

Operations that may block, execute external processes, perform I/O, or
require cancellation MUST take `context.Context` as the first parameter.
`context.Context` MUST NOT be stored in long-lived structs. Errors MUST
be returned explicitly and wrapped with enough context to diagnose the
failure. Panics MUST NOT be used for expected runtime or user-input
failures.

Go source MUST be formatted with `gofmt` and MUST pass `go vet`. Global
mutable state MUST NOT be introduced without a demonstrated need.

**Rationale**: Consistent Go practice keeps the engine reviewable,
testable, and safe under cancellation and failure.

### III. Testable Behavior (NON-NEGOTIABLE)

Behavior implemented from a specification MUST be testable. New behavior
and behavioral changes MUST be developed test-first:

1. Write or update a test that asserts the specified observable behavior.
2. Run it and confirm it fails because the behavior is missing or wrong.
3. Implement only enough to make that test pass.
4. Refactor while those tests stay green.

Tests MUST assert observable behavior rather than internal implementation
details. Production behavior MUST NOT be added before a failing test
exists for it. Mechanical changes that do not alter observable behavior
MAY proceed without a new test.

Regression fixes MUST start from a failing test that demonstrates the
defect when practical. Tests MUST NOT be weakened or removed merely to
make an implementation pass. The relevant `go test` suite MUST pass
before an implementation task is considered complete.

User stories in a specification MUST include an independent test
description. Features that change process execution, CLI contracts, or
other public interfaces MUST include integration or contract tests for
those boundaries, not only unit tests of internals.

**Rationale**: TRUSS executes user-defined pipelines; unverified behavior
is an operational risk, not a style preference. Test-first keeps the
specified outcome ahead of the implementation.

### IV. Maintainable Simplicity

Designs MUST be simple and explicit. Abstractions, packages, layers,
frameworks, or top-level directories MUST NOT be added solely for
hypothetical future requirements. Shortcuts that knowingly create
structural debt in core engine boundaries MUST NOT be taken.

Components MUST have clear responsibilities and explicit contracts. Core
execution logic MUST remain independent from presentation concerns such
as CLI output. External contracts MUST remain independent from
Go-specific implementation details so SDKs and integrations in other
languages can consume them.

Standard-library functionality MUST be preferred when it provides a
clear and maintainable solution. Third-party dependencies MUST solve a
concrete problem and justify their maintenance and supply-chain cost.
Changes MUST stay scoped to the active specification and its necessary
supporting work. Unrelated refactors MUST NOT be mixed into feature work.

**Rationale**: A portable runner stays evolvable only if the core stays
small, explicit, and free of speculative structure.

### V. Execution Safety and Portability

TRUSS executes user-defined commands and external processes. Implicit
command execution MUST NOT be introduced outside explicitly specified
pipeline behavior. Exit codes, cancellation, and errors MUST be
preserved accurately across execution layers. Subprocess failures MUST
NOT be swallowed.

Secrets MUST NOT be exposed through logs, diagnostics, process output
handling, or generated artifacts. Cross-platform behavior MUST be
specified and tested whenever process execution, filesystem behavior,
paths, signals, or shells are involved.

Public configuration formats and external contracts MUST be treated as
versioned interfaces. Incompatible changes to an established public
contract MUST identify compatibility impact before implementation.
Assumptions MUST NOT bind TRUSS to a specific CI provider, IDE, agent
platform, operating system, or SDK language unless a specification
explicitly requires it.

The product architecture MUST NOT depend on Claude Code or Spec Kit being
present at runtime.

**Rationale**: Correct, portable, secret-safe execution is the product.

## Go Module Constraints

- Core engine, CLI, and daemon code MUST be Go modules with packages
  that compile and test via `go test ./...`.
- SDKs and external integrations MAY use other languages and MUST NOT
  depend on Go-specific types, build tags, or module paths for their
  public contracts.
- Generated Spec Kit (`.specify/`) files and agent tooling (`.claude/`,
  `.agent/`, `scripts/`) are tooling. Project requirements MUST NOT be
  moved into them. Generated files MUST NOT be modified unless the task
  specifically concerns that tooling.
- `.specify/memory/constitution.md` is the source of truth for
  project-wide engineering principles. `AGENTS.md` provides agent
  workflow; it MUST NOT duplicate or silently override this constitution.

## Branching

- Every branch created for work in this repository MUST be named
  `<type>/<name>`, where `<type>` is exactly one of:
  - `feature/` for a new capability or a behavioral change, and for any
    other change that is not a `bugfix/` or a `hotfix/`, such as
    documentation, tests, or maintenance.
  - `bugfix/` for a non-urgent fix to a defect.
  - `hotfix/` for an urgent fix to a defect in released code.
- `<name>` MUST be short, lowercase, and hyphen-separated. For a feature
  that has a specification, `<name>` SHOULD be the spec directory name
  under `specs/`, such as `feature/003-step-dependencies`, so the branch
  and the specification match.
- Agents and contributors MUST NOT create a branch without one of these
  prefixes. Branches that already exist keep their names.

**Rationale**: A fixed prefix tells a reviewer the purpose of a branch
before opening it, and keeps branch names uniform across contributors and
agents.

## Agentic Operation

- Work MAY be done by coding agents under `.claude/` (agents, skills, rules,
  hooks, permissions). Agents are bound by this constitution exactly as
  contributors are; agent guidance MUST NOT weaken it.
- Agents MUST NOT merge, tag, publish a release, force-push, rewrite pushed
  history, or push to `develop` or `main`. A human reviews and merges every
  pull request.
- Review MUST be independent: the session that wrote a change does not
  review it.
- Durable agent knowledge lives in `.agent/` (memory, work state, event log).
  It MUST NOT contain secrets or personal data and MUST NOT restate this
  constitution or a specification; it links to them.
- Agent-authored changes to guardrails (`.claude/`, `.github/`, `scripts/`,
  this file) MUST be listed for the maintainer at the top of the pull request.

**Rationale**: Autonomy is acceptable only inside limits that tools enforce
and a human can audit.

## Quality Gates

A change is not complete until all of the following hold:

1. Implemented behavior matches the active specification, or any
   deviation is documented and accepted.
2. The implementation follows the technical plan, or intentional
   deviations are documented.
3. Specified tests were written first and failed before the behavior
   existed, and `go test` for the affected packages passes.
4. `gofmt` and `go vet` succeed for changed Go code.
5. Process-execution, cancellation, exit-code, and secret-handling
   requirements remain intact when those layers are touched.
6. Compatibility impact is identified for any public contract change.
7. Unresolved limitations are reported explicitly.

Complexity that violates a principle MUST be recorded in the plan's
Complexity Tracking table with the simpler alternative that was rejected.

## Governance

This constitution supersedes informal practice, contributor habit, and
agent defaults. Feature specifications, plans, tasks, and code MUST
comply. Where a specification needs an exception, the exception MUST be
explicit, scoped, and justified in that feature's plan.

Amendments MUST be made by editing this file, updating dependent
templates (`plan-template.md`, `spec-template.md`, `tasks-template.md`)
and agent guidance, and recording a Sync Impact Report. Versioning
follows semantic versioning:

- MAJOR: removal or incompatible redefinition of a principle or gate
- MINOR: new principle or materially expanded guidance
- PATCH: clarifications, wording, and non-semantic refinements

Compliance review is required for plans (Constitution Check),
implementation (Quality Gates), and any change to public contracts.
Use `AGENTS.md` for agent workflow only; do not treat it as a second
constitution.

**Version**: 1.3.0 | **Ratified**: 2026-09-16 | **Last Amended**: 2026-10-30
