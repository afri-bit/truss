---
# Lifecycle: draft -> approved -> implementing -> done
# (see docs/development/spec-driven-workflow.md)
status: draft
feature: "[###-feature-name]"
document: plan
---

# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go (core engine, CLI, daemon; version from go.mod) or NEEDS CLARIFICATION  
**Primary Dependencies**: Go standard library first; third-party only with justified need or NEEDS CLARIFICATION  
**Storage**: [if applicable, e.g., files, sqlite, or N/A]  
**Testing**: `go test ./...`; integration/contract tests for execution and public contracts  
**Target Platform**: Portable (local and CI; host or container per step) or NEEDS CLARIFICATION  
**Project Type**: CLI + engine + daemon (portable pipeline runner)  
**Performance Goals**: [domain-specific or N/A]  
**Constraints**: Execution safety, cancellation, exit-code fidelity, secret non-disclosure, cross-platform paths/processes  
**Scale/Scope**: [domain-specific or NEEDS CLARIFICATION]

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [ ] Spec exists and its status is `approved`; this plan does not invent unspecified behavior
- [ ] Import boundaries hold: `pkg/engine` imports nothing from daemon, API, or store implementations
- [ ] The engine emits typed events only; formatting stays in renderers and sinks
- [ ] Changes to a public contract (pipeline schema, CLI surface, HTTP API `/v1`) link an ADR
- [ ] Core execution remains independent from CLI/presentation
- [ ] External contracts do not leak Go-specific types or conventions
- [ ] Process execution, cancellation, exit codes, and secret handling are explicit where touched
- [ ] Tests cover specified behavior (`go test`); integration/contract tests included when execution or public contracts change
- [ ] Public contract changes identify compatibility impact
- [ ] No speculative packages, layers, frameworks, or dependencies
- [ ] Cross-platform behavior is specified for paths, processes, signals, or shells
- [ ] Complexity Tracking records any principle violation and the simpler alternative rejected

Violations MUST be resolved or justified in Complexity Tracking before Phase 0 proceeds.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: List only the packages this feature touches. The layout
  and import rules are defined in docs/architecture.md; do not add new
  top-level directories or packages without justifying them here.
-->

```text
cmd/truss/                 # CLI and daemon entrypoint (composition root)
pkg/
├── pipeline/              # schema, parser, validator (innermost)
├── engine/                # sequential step lifecycle, typed events
├── executor/              # host (os/exec) and container (Docker) backends
├── store/                 # run-history contract; implementations in sub-packages
└── api/                   # HTTP /v1 types and client
internal/
├── daemon/                # scheduler, binding registry, HTTP + SSE server
├── socket/                # Unix socket resolution and permissions
└── sinks/                 # EventSink implementations (renderers, recorder, SSE)
# Tests live beside code as *_test.go; fixtures in testdata/.
# Docker-dependent tests use the `integration` build tag.
```

**Structure Decision**: [Packages touched and why]

## Design

### Approach

[How the feature works, in a few paragraphs. Reference FR IDs from spec.md.]

### Interfaces and Type Signatures

```go
// Exported types, interfaces (defined at the consumer), and function
// signatures this feature adds or changes.
```

### Data Flow

[Inputs → transformations → outputs; events emitted and their order. A
Mermaid diagram is welcome.]

### Error Handling

[Error types and sentinels, wrapping, what reaches the user and how, exit
codes, cancellation behavior.]

### Cross-Platform Concerns

[Paths (`filepath`), line endings, process and signal semantics, shells,
file permissions, and socket behavior on Linux, macOS, and Windows.]

### Test Strategy

[Unit tests (table-driven), golden files, integration tests behind the
`integration` tag, and which FR IDs each test covers.]

### Public Contract Impact

[None, or the affected contract (pipeline schema, CLI surface, HTTP API
`/v1`), compatibility impact, and the ADR that records the decision.]

### Alternatives Considered

| Alternative | Why rejected |
|-------------|--------------|
| [Option] | [Reason] |

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
