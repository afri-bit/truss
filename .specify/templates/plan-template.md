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

- [ ] Spec exists; this plan does not invent unspecified behavior
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
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Go module (DEFAULT for TRUSS core)
cmd/
├── truss/          # CLI entrypoint
└── trussd/         # daemon entrypoint (if in scope)
internal/
├── engine/         # core execution (independent of CLI presentation)
├── pipeline/       # pipeline model and scheduling
└── exec/           # process execution boundaries
# Public Go API only if intentionally supported:
# pkg/...
# Tests live beside code as *_test.go; testdata/ for fixtures.

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
