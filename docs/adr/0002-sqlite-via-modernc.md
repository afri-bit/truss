---
status: accepted
date: 2026-10-07
decision-makers: [afri-bit]
---

# Persist run history in SQLite via modernc.org/sqlite (no CGO)

## Context and Problem Statement

The daemon must persist bindings and run history (runs, step results, logs
metadata) across restarts. Direct mode needs the same contract in memory.
Which storage engine fits a single, cross-compiled binary?

## Decision Drivers

- Embedded: no external database service to install or run
- `CGO_ENABLED=0` so every platform builds from one host (ADR-0001)
- Transactional and crash-safe
- Queryable for `truss status` and `truss logs`

## Considered Options

- SQLite via `modernc.org/sqlite` (pure Go translation)
- SQLite via `mattn/go-sqlite3` (CGO)
- Embedded key-value store (bbolt)
- Flat JSON files

## Decision Outcome

Chosen option: "SQLite via `modernc.org/sqlite`", because it is the only
option that is both SQL-queryable and buildable without a C toolchain.
Persistence sits behind the `store.Store` contract in `pkg/store`, with
`SQLiteStore` for the daemon and `MemStore` for direct mode.

### Consequences

- Good, because cross-compilation stays trivial and release builds need no
  C toolchain.
- Good, because the schema is inspectable with any SQLite tool.
- Bad, because the pure-Go driver is slower than the CGO driver. Run
  history volume is small, so this is acceptable.
- Bad, because the driver is a large dependency, which adds to binary size.

### Confirmation

Release builds use `CGO_ENABLED=0`. depguard forbids `modernc.org/sqlite`
in `pkg/engine`. The `Store` contract has a shared test suite run against
both implementations.

## Pros and Cons of the Options

### mattn/go-sqlite3

- Good, because it is fast and battle-tested.
- Bad, because it requires CGO and a C cross-compiler per target.

### bbolt

- Good, because it is pure Go and simple.
- Bad, because there is no query language; listing and filtering runs
  means hand-written indexes.

### JSON files

- Bad, because there is no atomicity across files, and concurrent writers
  corrupt data.

## More Information

Related: ADR-0001.
