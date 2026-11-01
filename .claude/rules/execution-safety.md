---
paths:
  - "internal/executor/**"
  - "internal/engine/**"
  - "internal/runlog/**"
  - "cmd/**"
  - ".github/scripts/**"
---

# Execution safety and portability

Constitution principle V. TRUSS runs user-defined commands, so mistakes here are expensive.

- No implicit command execution outside specified pipeline behavior. Every process started is a
  step the user declared.
- Preserve exit codes, cancellation and errors across layers. Never swallow a subprocess failure.
- A step that cannot start is reported as a diagnostic with no exit code (see the run-log spec).
- Secrets never reach logs, diagnostics, process-output handling or generated artifacts. The run
  log never lists environment variable names or values.
- Anything touching processes, filesystem, paths, signals or shells states its behavior for Linux,
  macOS and Windows and has a test or CI coverage for each. Use `filepath`, not string paths.
  A `run` string is a command for the host's shell; do not promise cross-shell portability.
- `gosec` exclusions in `.golangci.yml` are deliberate and narrow. Do not widen them to silence a
  finding; fix it or explain it in the plan's Complexity Tracking table.
- Do not bind TRUSS to a CI provider, IDE, agent platform or OS unless a spec requires it. The
  product must not depend on Claude Code or Spec Kit at runtime.
