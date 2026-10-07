---
status: accepted
date: 2026-10-07
decision-makers: [afri-bit]
---

# Use HTTP with Server-Sent Events over a Unix domain socket between CLI and daemon

## Context and Problem Statement

The CLI must control the daemon (bind pipelines, trigger runs, query status)
and follow live run events and logs. Which transport and protocol should
the CLI ↔ daemon API (`/v1`, a public contract) use?

## Decision Drivers

- Local-only by default: no network port, access controlled by file
  permissions
- Works on Linux, macOS, and Windows
- Streaming of run events and log lines to `truss logs --follow`
- Debuggable with standard tools (`curl --unix-socket`)
- Usable from non-Go clients and a future web UI (v0.3)

## Considered Options

- HTTP/1.1 + SSE over a Unix domain socket
- gRPC over a Unix domain socket
- HTTP over TCP localhost
- Custom line protocol

## Decision Outcome

Chosen option: "HTTP/1.1 + SSE over a Unix domain socket", because it is
local-only, permission-controlled, needs only the standard library, and
remains a plain HTTP API that other clients and a future web UI can reuse
over TCP. All routes are versioned under `/v1`.

### Consequences

- Good, because there is no open port by default, and socket file
  permissions restrict access to the owning user.
- Good, because SSE is one-directional streaming that is simple to
  implement and consume, with reconnect via `Last-Event-ID`.
- Good, because request and response types in `pkg/api` are plain JSON
  and language-neutral.
- Bad, because Windows supports `AF_UNIX` only from Windows 10 1803 /
  Server 2019. Older Windows versions are unsupported.
- Bad, because SSE is server-to-client only; control actions are separate
  requests.

### Confirmation

`internal/socket` owns path resolution and permissions. The `/v1` route
table and payloads are covered by contract tests. Changes require a spec
and an ADR.

## Pros and Cons of the Options

### gRPC

- Good, because of typed streaming and code generation.
- Bad, because of heavy dependencies, a protobuf toolchain, and poorer
  debuggability with curl. Browser support also needs a proxy.

### HTTP over TCP localhost

- Good, because it is universally supported.
- Bad, because any local user or process can reach the port; it needs
  authentication from day one.

### Custom protocol

- Bad, because it means reinventing framing, versioning, and tooling.

## More Information

Related: ADR-0001. A TCP listener for the web UI is out of scope for
v0.1.0 and needs its own ADR.
