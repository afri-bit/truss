# Architecture Decision Records

Significant architectural decisions for TRUSS, recorded in
[MADR 4.0](https://adr.github.io/madr/) format. Use the `write-adr` skill or
copy [`template.md`](template.md).

Rules:

- Number sequentially (`NNNN-short-title.md`); never reuse a number.
- Changes to a public contract (pipeline schema, CLI surface, HTTP API
  `/v1`) require an ADR alongside the approved spec.
- Accepted ADRs are immutable. To change a decision, write a new ADR and
  mark the old one `superseded by ADR-NNNN`.

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-go-single-binary.md) | Implement TRUSS in Go and ship CLI and daemon as a single binary | accepted |
| [0002](0002-sqlite-via-modernc.md) | Persist run history in SQLite via modernc.org/sqlite (no CGO) | accepted |
| [0003](0003-unix-socket-http-sse.md) | Use HTTP with SSE over a Unix domain socket between CLI and daemon | accepted |
| [0004](0004-typed-engine-events.md) | The engine emits typed events; sinks and renderers own formatting | accepted |
| [0005](0005-sequential-execution-v0-1.md) | Execute steps sequentially only in v0.1.0 | accepted |
