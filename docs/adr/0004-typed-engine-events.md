---
status: accepted
date: 2026-10-07
decision-makers: [afri-bit]
---

# The engine emits typed events; sinks and renderers own formatting

## Context and Problem Statement

The same run must be shown live in a terminal (direct mode), persisted to
SQLite, and streamed over SSE to `truss logs --follow`, with a web UI to
come later. How should the engine communicate progress so that every
consumer gets complete, structured information?

## Decision Drivers

- Several independent consumers with different output formats
- Testability: assert on what happened, not on printed text
- The engine must not depend on presentation or transport (dependency
  rule 1)
- Stable, serializable payloads for the `/v1` SSE stream

## Considered Options

- Typed event values delivered to an `EventSink` interface
- Engine writes formatted log lines to an `io.Writer`
- Engine logs through `slog` and consumers parse records
- Callback per lifecycle hook (`OnStepStart`, `OnStepEnd`, …)

## Decision Outcome

Chosen option: "Typed event values delivered to an `EventSink`", because
it decouples the engine from every consumer and makes behavior directly
assertable in tests.

The engine defines event types (for example run started, step started,
step output, step finished, run finished) carrying IDs, timestamps, status,
and exit codes. It emits them, in order, to an `EventSink` that the caller
supplies. `internal/sinks` provides the terminal renderer, the store
recorder, and the SSE broadcaster, and a fan-out sink combines them.

### Consequences

- Good, because new outputs (JSON lines, web UI) need no engine changes.
- Good, because tests use an in-memory sink and compare event slices.
- Bad, because event types become part of the HTTP API contract when
  serialized, so changes need care and versioning.
- Bad, because sinks must not block the engine; buffering and
  back-pressure policy must be specified.

### Confirmation

forbidigo forbids `fmt.Print*`, `os.Stdout`, and `os.Stderr` in `pkg/`.
depguard forbids the engine from importing sinks, store, or API packages.
`code-reviewer` checks for string-typed events.

## Pros and Cons of the Options

### Formatted lines to an `io.Writer`

- Good, because it is simplest at first.
- Bad, because every consumer re-parses text, and changing the format
  breaks consumers.

### slog records

- Good, because the infrastructure already exists.
- Bad, because logs are diagnostics, not a domain contract; attribute
  types are loosely enforced.

### Lifecycle callbacks

- Good, because they are typed.
- Bad, because the interface grows with every new event, and fan-out and
  serialization still need a value type.

## More Information

Related: ADR-0003 (SSE transport).
