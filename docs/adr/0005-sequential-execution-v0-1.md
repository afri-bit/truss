---
status: accepted
date: 2026-10-07
decision-makers: [afri-bit]
---

# Execute steps sequentially only in v0.1.0

## Context and Problem Statement

Pipelines eventually benefit from dependency graphs and parallel steps.
Supporting them affects the schema (`needs:`), the engine, event ordering,
log interleaving, and cancellation. Should v0.1.0 support them?

## Decision Drivers

- Ship a small, correct, portable core first
- Deterministic event and log ordering for the first renderers and tests
- Keep the v1 schema extensible without breaking changes

## Considered Options

- Sequential only; steps run in declared order
- DAG with parallel execution from the start
- Sequential now, with the `needs:` field reserved in the v1 schema

## Decision Outcome

Chosen option: "Sequential only", because it keeps the engine, the event
model, and failure semantics simple while the contracts settle. In v0.1.0,
steps run in declaration order, and the first failing step stops the run.

Whether to reserve `needs:` (or similar) in the v1 schema is left to the
pipeline schema spec (001). Adding an optional field later is a compatible
change to `version: 1`.

### Consequences

- Good, because the event stream is totally ordered, which makes renderers
  and tests trivial.
- Good, because cancellation and failure semantics are easy to specify.
- Bad, because long pipelines with independent steps run slower than they
  could.
- Bad, because DAG support later needs a spec, an ADR, and engine changes
  (planned after v0.1.0).

### Confirmation

Spec 001 rejects unknown keys, so DAG fields cannot be used silently.
`spec-reviewer` flags DAG or parallel features in v0.1.0 specs as scope
creep.

## More Information

Related: ADR-0004 (event ordering).
