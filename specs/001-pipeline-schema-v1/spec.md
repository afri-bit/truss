---
# Lifecycle: draft -> approved -> implementing -> done
# (see docs/development/spec-driven-workflow.md)
status: draft
feature: "001-pipeline-schema-v1"
document: spec
---

# Feature Specification: Pipeline Schema v1

**Feature Branch**: `001-pipeline-schema-v1`  
**Created**: 2026-10-07  
**Input**: User description: "Define the v1 YAML pipeline schema, its
validation rules, and error messages that point at the offending line."

## Problem *(mandatory)*

TRUSS promises that one pipeline file runs identically on a laptop, on
Jenkins, and on GitHub Actions. That promise starts with the file itself.
Today there is no defined schema. Authors cannot know which keys are valid,
and tools cannot reject a broken file before it starts running commands.

Pipeline authors need a small, strict, versioned format. They need typos and
structural mistakes reported up front, all at once, with the exact line and
column, the way a compiler reports errors. The schema is one of TRUSS's three
public contracts. Once released, it must stay stable and evolve only through
versioning.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Define a pipeline that TRUSS accepts (Priority: P1)

A developer writes `pipeline.yml` with a version, a pipeline name, and an
ordered list of steps. TRUSS loads it into a validated pipeline model
without executing anything.

**Why this priority**: Nothing else in TRUSS works without a parseable
pipeline. This is the entry point for the engine, the daemon, and the CLI.

**Independent Test**: Load each valid fixture under the feature's test data.
Assert that the resulting model has the expected name, step IDs, step order,
executors, images, commands, and environment, and that no process was
started.

**Acceptance Scenarios**:

1. **Given** a file with `version: 1`, a `pipeline.name`, and two host
   steps, **When** it is loaded, **Then** the model contains both steps in
   declaration order with their `run` text preserved byte-for-byte.
2. **Given** a valid file that mixes host and container steps, **When** it
   is loaded, **Then** each step reports its declared executor, and
   container steps report their image.

---

### User Story 2 - See every mistake at once, with its location (Priority: P1)

A developer makes several mistakes, such as a misspelled key, a duplicate
step ID, and a container step without an image. TRUSS reports all of them in
one pass. Each report shows the file, line, column, field path, and a
message that says what is wrong and what is expected.

**Why this priority**: Fast, precise feedback is the core of "edit, run
locally, fix". Reporting only the first error, or reporting errors without
positions, forces repeated round trips.

**Independent Test**: For each invalid fixture, compare the full list of
reported errors against a golden file. The golden file records position,
field path, and message.

**Acceptance Scenarios**:

1. **Given** a step with `exectuor: host`, **When** the file is validated,
   **Then** one error points at the key `exectuor` and suggests `executor`.
2. **Given** two steps with `id: build`, **When** the file is validated,
   **Then** one error points at the second `id` value and names the line of
   the first.
3. **Given** a file with three independent mistakes, **When** it is
   validated, **Then** three errors are reported, ordered by line and then
   column.

---

### User Story 3 - Declare where each step runs (Priority: P2)

A developer marks each step to run either on the host or in a container
image. The decision is visible in the file, not inferred.

**Why this priority**: Host and container execution are both in v0.1.0
scope. The executor choice must be explicit for the engine to honor it.

**Independent Test**: Validate fixtures covering host steps, container
steps, missing images, and images on host steps. Assert the model contents
or the reported errors.

**Acceptance Scenarios**:

1. **Given** `executor: container` with `image: golang:1.27`, **When** the
   file is validated, **Then** it is accepted.
2. **Given** `executor: container` without `image`, **When** the file is
   validated, **Then** an error points at the step and states that `image`
   is required for container steps.

---

### User Story 4 - Same file, same verdict on every platform (Priority: P3)

A team edits the same pipeline on Windows and Linux. The file may have CRLF
line endings or a UTF-8 byte-order mark. Validation gives identical results
and identical positions everywhere.

**Why this priority**: Cross-platform behavior is a product feature. It is
cheap to guarantee now and expensive to retrofit.

**Independent Test**: Run the full fixture suite on Linux, macOS, and
Windows in CI. Also run each fixture again with CRLF line endings, and with
a leading UTF-8 BOM. Assert identical models and identical error lists.

**Acceptance Scenarios**:

1. **Given** a fixture converted to CRLF line endings, **When** it is
   loaded, **Then** the model and any errors equal those of the LF version.

---

### Edge Cases

- Empty file, or a file that contains only comments.
- A file with more than one YAML document (`---` separators).
- Malformed YAML: a tab used for indentation, an unclosed quote, or a bad
  indent.
- A duplicate mapping key in the same mapping (for example, two `run:` keys
  in one step).
- The wrong node kind: `steps` is a mapping instead of a sequence, or `env`
  is a sequence.
- `steps: []` (an empty list).
- YAML anchors and aliases, used to share `env` blocks between steps.
- Values that YAML would type implicitly, such as `PORT: 8080` or
  `DEBUG: yes` in `env`.
- Multi-line `run` blocks (`|` and `>`), including trailing newline
  handling.
- Non-ASCII characters before the error column, which affect how the column
  is counted.
- `workdir` values that are absolute, use `..` to escape the workspace, or
  use Windows separators.

## Requirements *(mandatory)*

### Schema (normative summary)

```yaml
version: 1                     # required; schema version
pipeline:                      # required
  name: build-and-test         # required; identifier
  description: Optional text   # optional
env:                           # optional; applies to every step
  GOFLAGS: -mod=readonly
steps:                         # required; at least one; run in order
  - id: unit-tests             # required; unique identifier
    name: Unit tests           # optional; display name
    executor: host             # required; host | container
    run: go test ./...         # required; command text
    env:                       # optional; overrides pipeline env per key
      CGO_ENABLED: "0"
    workdir: ./src             # optional; relative to the workspace
  - id: lint
    executor: container
    image: golangci/golangci-lint:v2.14.0   # required iff executor: container
    run: golangci-lint run ./...
```

How `run` is interpreted (shell, argument vector, or platform default) is
an execution concern and is not decided here, but see FR-020.

### Functional Requirements (Acceptance Criteria)

**Document and version**

- **FR-001**: WHEN a pipeline file is loaded, THE SYSTEM SHALL parse and
  validate it without executing any command, starting any process, or
  contacting any container runtime.
- **FR-002**: IF the file is empty or contains only comments, THEN THE
  SYSTEM SHALL report an error stating that the pipeline file is empty.
- **FR-003**: IF the file contains more than one YAML document, THEN THE
  SYSTEM SHALL report an error at the start of the second document.
- **FR-004**: IF the file is not well-formed YAML, THEN THE SYSTEM SHALL
  report the syntax error with the line and column where the parser
  failed.
- **FR-005**: IF the top-level `version` key is missing, THEN THE SYSTEM
  SHALL report an error stating that `version` is required and listing the
  supported versions.
- **FR-006**: IF `version` is present but is not a supported schema
  version, THEN THE SYSTEM SHALL report an error at the value that names
  the unsupported value and the supported versions.
  [NEEDS CLARIFICATION: is the only accepted form the integer `1`, or must
  the string `"1"` also be accepted? The README and glossary currently
  show `version: "1"`.]

**Structure**

- **FR-007**: IF a mapping contains a key that the schema does not define
  at that location, THEN THE SYSTEM SHALL report an error at the key.
- **FR-008**: WHEN an unknown key is within edit distance 2 of a valid key
  at the same location, THE SYSTEM SHALL include "did you mean
  `<valid key>`?" in that error.
- **FR-009**: IF the same key appears twice in one mapping, THEN THE SYSTEM
  SHALL report an error at the second occurrence that names the line of
  the first.
- **FR-010**: IF a value has the wrong YAML node kind (mapping, sequence, or
  scalar) for its field, THEN THE SYSTEM SHALL report an error at the value
  that names the expected kind.
- **FR-011**: IF a required field (`version`, `pipeline`, `pipeline.name`,
  `steps`, `steps[*].id`, `steps[*].executor`, or `steps[*].run`) is
  missing, THEN THE SYSTEM SHALL report an error at the enclosing mapping
  that names the missing field.

**Pipeline**

- **FR-012**: IF `pipeline.name` is not 1–64 characters drawn from ASCII
  letters, digits, `-`, `_`, and `.`, beginning with a letter or digit, THEN
  THE SYSTEM SHALL report an error at the value that states the allowed
  format.
- **FR-013**: IF `steps` is an empty sequence, THEN THE SYSTEM SHALL report
  an error stating that at least one step is required.

**Steps**

- **FR-014**: WHEN a pipeline is valid, THE SYSTEM SHALL expose its steps in
  declaration order.
- **FR-015**: IF a step `id` is not 1–64 characters matching
  `^[a-z][a-z0-9_-]*$`, THEN THE SYSTEM SHALL report an error at the value
  that states the allowed format.
- **FR-016**: IF two steps share the same `id`, THEN THE SYSTEM SHALL report
  an error at the later `id` that names the line of the earlier one.
- **FR-017**: IF `executor` is not `host` or `container`, THEN THE SYSTEM
  SHALL report an error at the value that lists the allowed executors.
  [NEEDS CLARIFICATION: must every step declare `executor`, or does an
  omitted `executor` default to `host`?]
- **FR-018**: IF a step declares `executor: container` without a non-empty
  `image`, THEN THE SYSTEM SHALL report an error at the step stating that
  `image` is required for container steps.
- **FR-019**: IF a step declares `executor: host` together with `image`,
  THEN THE SYSTEM SHALL report an error at `image` stating that `image` is
  only valid for container steps.
- **FR-020**: IF `run` is empty or contains only whitespace, THEN THE SYSTEM
  SHALL report an error at the value. [NEEDS CLARIFICATION: is `run` always
  a single string interpreted by a shell (and if so, which shell on each
  OS, and is a `shell` field part of v1), or may it also be a list of
  arguments executed without a shell?]
- **FR-021**: WHEN `run` uses a YAML block scalar, THE SYSTEM SHALL
  preserve its content exactly as YAML defines it, including newlines
  between lines.

**Environment and working directory**

- **FR-022**: IF an `env` key (pipeline or step level) does not match
  `^[A-Za-z_][A-Za-z0-9_]*$`, THEN THE SYSTEM SHALL report an error at the
  key.
- **FR-023**: WHEN an `env` value is a scalar, THE SYSTEM SHALL use the
  scalar's text exactly as written in the source as the value (for example,
  `8080`, `yes`, and `true` stay those literal strings) without implicit
  type conversion.
- **FR-024**: IF an `env` value is null, a mapping, or a sequence, THEN THE
  SYSTEM SHALL report an error at the value.
- **FR-025**: WHEN a step and the pipeline both define the same `env` key,
  THE SYSTEM SHALL use the step's value for that step.
- **FR-026**: THE SYSTEM SHALL NOT expand variables or expressions in any
  field (no `$VAR`, `${{ }}`, or templating); values are literal.
- **FR-027**: IF `workdir` is an absolute path (on any supported OS, for
  example `/x`, `C:\x`, or `\\host\share`), contains a `..` segment, or
  contains a backslash, THEN THE SYSTEM SHALL report an error at the value
  stating that `workdir` must be a relative, forward-slash path inside the
  workspace.

**Error reporting**

- **FR-028**: WHEN validation finds errors, THE SYSTEM SHALL report all of
  them in a single pass rather than stopping at the first.
- **FR-029**: WHEN reporting errors, THE SYSTEM SHALL order them by line,
  then column, then message.
- **FR-030**: THE SYSTEM SHALL format each error as
  `<file>:<line>:<column>: <field path>: <message>`. `<file>` is the path as
  the user supplied it. Line and column are 1-based. `<field path>` uses
  dotted keys and zero-based indices (for example, `steps[2].image`).
  Errors not tied to a field omit the field path segment.
- **FR-031**: THE SYSTEM SHALL count columns in Unicode code points, with a
  tab counting as one column.
- **FR-032**: WHEN an error concerns a missing field, THE SYSTEM SHALL
  report the position of the enclosing mapping's first key.
- **FR-033**: WHEN an error concerns a value reached through a YAML alias,
  THE SYSTEM SHALL report the position of the alias, not the anchor.
- **FR-034**: WHEN validation fails, THE SYSTEM SHALL return no pipeline
  model, so that no step can run from an invalid file.

**Portability**

- **FR-035**: WHEN a file uses CRLF line endings or begins with a UTF-8
  byte-order mark, THE SYSTEM SHALL produce the same model and the same
  error positions as for the equivalent LF file without a BOM.
- **FR-036**: IF the file is not valid UTF-8, THEN THE SYSTEM SHALL report
  an error with the line and column of the first invalid byte sequence.

### Quality, Safety & Compatibility *(mandatory when applicable)*

- **Testing**: Each FR is covered by at least one fixture. Valid fixtures
  are asserted against expected models, and invalid fixtures are compared
  against golden error lists. The suite runs on Linux, macOS, and Windows.
- **Process execution**: Loading and validation never execute commands
  (FR-001). The executors and the engine are out of scope.
- **Secrets**: Validation errors may quote keys and the offending value
  only for `version`, `executor`, `id`, `pipeline.name`, and unknown keys.
  Error messages SHALL NOT echo `env` values or `run` content, which may
  contain secrets.
- **Cross-platform**: Covered by FR-027, FR-031, FR-035, and FR-036, and by
  the three-OS test matrix.
- **Compatibility**: This spec defines public contract v1. After approval
  and release, removing fields, tightening validation, or changing meaning
  needs `version: 2`. Adding optional fields is a compatible change that
  still requires a spec and an ADR. Error message wording is not part of
  the contract, but the `<file>:<line>:<column>:` prefix is.
- **Portability**: The schema has no provider-specific concepts. Triggers
  and schedules belong to daemon bindings, not to the pipeline file.

### Key Entities

- **Pipeline**: a name, an optional description, pipeline-level
  environment, and an ordered, non-empty list of steps.
- **Step**: a unique ID, an optional display name, an executor (host or
  container), an image (container only), command text, step-level
  environment, and an optional working directory.
- **Validation Error**: a source position (file, line, column), an optional
  field path, and a human-readable message.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every FR is traced to at least one passing test, and every
  test names the FR IDs it covers.
- **SC-002**: The fixture suite produces byte-identical results on Linux,
  macOS, and Windows CI runners.
- **SC-003**: For every invalid fixture, the first reported position is
  within the token the golden file identifies as the cause (zero
  off-by-one errors).
- **SC-004**: Validating a 500-step pipeline completes in under 100 ms on
  a standard CI runner.

## Non-Goals *(mandatory)*

- Executing steps: interpreting `run`, process lifecycle, exit codes,
  timeouts, and cancellation (engine and executor specs).
- Dependency graphs, `needs:`, parallel steps, or matrices (ADR-0005; after
  v0.1.0).
- Secrets references, caching, artifacts, conditionals (`if:`), includes,
  templates, and expressions.
- Triggers, schedules, and workspaces inside the pipeline file. These
  belong to daemon bindings.
- Importing GitHub Actions or Jenkins definitions.
- Publishing a JSON Schema file or editor integration (may follow in a
  later spec).
- The CLI command that surfaces these errors, and its exit codes (the CLI
  surface spec).

## Assumptions

- Pipelines are authored by hand and are small (fewer than 1,000 steps,
  under 1 MiB).
- The file path is supplied by the caller (CLI or daemon binding). Path
  discovery rules (for example, a default `pipeline.yml`) belong to the CLI
  spec.
- YAML 1.2 core schema semantics apply. Implicit typing is neutralized for
  `env` values by FR-023.
- Anchors and aliases are allowed. Merge keys (`<<`) are treated as
  ordinary keys and therefore rejected as unknown keys.
