# 0013. macOS resource figures through libSystem, without cgo

- Status: Proposed
- Date: 2026-11-13

## Context

[ADR 0012](0012-daemon-samples-resources-with-the-standard-library.md) left macOS without
resource figures: it has no `/proc`, the standard library exposes no call for CPU ticks or
another process's usage, and running `ps` every sample was rejected. The maintainer asked for
macOS figures. The release builds with `CGO_ENABLED=0`.

## Decision

`internal/metrics` calls libSystem directly, the way `golang.org/x/sys/unix` does on macOS: a
`//go:cgo_import_dynamic` line per function, an assembly trampoline per function
(`libsystem_darwin.s`, the same text for amd64 and arm64), and the runtime's `syscall.syscall`
reached through `//go:linkname`, which the runtime keeps for this use (go.dev/issue/67401).

- Host CPU and cores: `host_processor_info`. Memory used: `host_statistics64`, counted like
  Activity Monitor's "Memory Used" (app + wired + compressed). Total: `hw.memsize`. Load:
  `vm.loadavg`.
- A step: its whole process group through `proc_listpids`, each process through `proc_pidinfo`
  (CPU, resident size, threads) and `proc_pid_rusage` (CPU of children it waited for), like Linux.
- CPU times come in Mach absolute units and are converted with `mach_timebase_info`. Page
  counts are in the kernel's page size (`host_page_size`), which is not the task's page size
  when an amd64 build runs under Rosetta.
- Struct decoding is plain Go with no build constraint, so it is tested on every system.

Rejected: `golang.org/x/sys/unix` (a new module, and it wraps none of these calls); cgo (breaks
cross-compiled releases); `ps` or `top` (a process every sample, already rejected in 0012).

## Consequences

- Good: macOS reports the same fields as Linux, `tree: true` included. No new module.
- Bad: about 150 lines of low-level code tied to C struct layouts and to a runtime linkname. A
  future Go release that removed `syscall.syscall` would break the macOS build, loudly; x/sys
  would break with it.
- A function missing from libSystem would stop every `truss` command from starting on macOS,
  not only the figures. CI therefore builds the end-to-end binary with `CGO_ENABLED=0`, as the
  release does, and its daemon samples on the `macos-latest` runner.
- Follow-ups: CI runs Apple silicon only; no Intel Mac and no Rosetta run is tested.
