---
name: "verify"
description: "Run TRUSS's quality gate (gofmt, go vet, golangci-lint, go test -race, memory check, and the end-to-end script when the CLI changed) and report a compact result. Use before pushing or opening a PR."
argument-hint: "Optional: 'e2e' to include the built-binary end-to-end script"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Verify

Delegate to the `check-runner` agent so long logs stay out of the main conversation.

1. Include the end-to-end script if the input says `e2e`, or if `git diff develop...HEAD --name-only`
   shows changes under `cmd/`, `internal/executor/`, `internal/runlog/`, `.github/scripts/`, or the
   built binary's behavior may change. Include `release-check_test.sh` if release scripts changed.
2. Spawn `check-runner` with the checks to run (see its definition).
3. Relay its summary. If something failed, show the first actionable errors and propose the fix, but
   never disable, skip or weaken a check or a test.
4. If a tool is missing locally (for example `golangci-lint` or the required Go toolchain), say so
   plainly; CI on Linux, macOS and Windows remains the authority.
