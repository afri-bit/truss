---
status: accepted
date: 2026-10-07
decision-makers: [afri-bit]
---

# Implement TRUSS in Go and ship CLI and daemon as a single binary

## Context and Problem Statement

TRUSS must run identically on developer machines and CI agents across
Linux, macOS, and Windows, with no runtime to install first. It needs a CLI
(`truss`) and a long-running daemon (`trussd`). Which language and
distribution shape gives us portable, dependency-free installation?

## Decision Drivers

- Zero-install on CI runners and laptops: one file, no interpreter or VM
- First-class cross-compilation for linux, darwin, windows on amd64/arm64
- Strong standard library for processes, HTTP, and filesystems
- Mature Docker Engine SDK for the container executor
- Contributor familiarity in the CI and infrastructure space

## Considered Options

- Go, single static binary with a `daemon` subcommand
- Go, two binaries (`truss` and `trussd`)
- Rust, single binary
- Node.js or Python, distributed via a package manager

## Decision Outcome

Chosen option: "Go, single binary with a `daemon` subcommand", because it
satisfies every driver and halves the release and version-skew surface: the
CLI and the daemon that serves it always come from the same build.
`trussd` is `truss daemon`; packagers may add a `trussd` symlink or alias.

### Consequences

- Good, because one artifact per platform keeps installation and release
  simple and makes CLI/daemon version skew impossible on a single machine.
- Good, because `CGO_ENABLED=0` builds cross-compile from any host (see
  ADR-0002).
- Bad, because the CLI binary carries daemon code (scheduler, HTTP server,
  SQLite driver), increasing its size.
- Bad, because Go's error handling and lack of sum types require discipline
  that the `go-conventions` skill and linters must enforce.

### Confirmation

`cmd/truss` is the only `main` package. Release builds use
`CGO_ENABLED=0` for every platform target.

## Pros and Cons of the Options

### Two Go binaries

- Good, because each binary is smaller.
- Bad, because users can install mismatched versions; the API contract then
  needs negotiation from day one.

### Rust

- Good, because of memory safety without GC and a strong type system.
- Bad, because of slower iteration, a less mature Docker client, and a
  smaller contributor pool for this domain.

### Node.js or Python

- Bad, because they need a runtime on every runner, which defeats
  "run anywhere unchanged".

## More Information

Related: ADR-0002 (no CGO), ADR-0003 (CLI ↔ daemon transport).
