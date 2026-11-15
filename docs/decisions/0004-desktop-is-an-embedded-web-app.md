# 0004. TRUSS Desktop is a web application embedded in the binary

- Status: Accepted
- Date: 2026-11-12

## Context

The maintainer wants a GUI, "TRUSS Desktop": dashboard, running jobs, a control panel, logs.
The usual routes are a native shell around web content (Electron, Tauri, Wails) or a native
toolkit. Each adds a toolchain, large dependencies, per-OS packaging and signing, and a second
release artifact, before the first screen exists.

## Decision

Build TRUSS Desktop as static HTML, CSS and JavaScript, embedded with `go:embed` and served by
the daemon. `truss desktop` opens it in the default browser. No framework, no build step, no
JavaScript dependencies. It uses the public HTTP API only.

## Consequences

- It ships today, in the same binary, on every OS the daemon supports.
- It cannot have a tray icon, native notifications or its own window yet.
- Because it only uses the public API, wrapping it later in a native shell (Wails or Tauri are
  the candidates) is packaging work, not a rewrite.
- Hand-written DOM code is fine at this size; if the UI grows past a few views, revisit a
  small framework in a new decision.
- A strict Content-Security-Policy (`default-src 'self'`) applies; the UI never injects text
  as markup.
