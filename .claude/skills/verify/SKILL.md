---
name: "verify"
description: "Run TRUSS's quality gate with scripts/verify.sh (gofmt, go vet, golangci-lint for Linux, Windows and macOS files, go test -race, agent tooling and script tests, and the end-to-end script when the CLI changed) and report the result. Use before pushing or opening a PR."
argument-hint: "Optional: 'e2e' to force the built-binary end-to-end script"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Verify

The gate is a script, so its result is the same every time and does not depend on an agent's
summary.

1. Run `bash scripts/verify.sh` (add `--e2e` if the input says `e2e`). It decides on its own whether
   the end-to-end script is needed (changes under `cmd/`, `internal/executor/`, `internal/runlog/` or
   `.github/scripts/`).
2. Relay its summary table as printed, including the `commit` and `result` lines. Paste the same
   lines into the PR under "Checks".
3. If a check failed, show the first actionable errors from its output and propose the fix. Never
   disable, skip or weaken a check or a test.
4. A `missing` tool is a failure, not a pass. In a cloud session, `golangci-lint` comes from the
   environment's setup script; if it is missing, install it as the script's message says and run
   again. CI on Linux, macOS and Windows remains the authority for code paths of other systems.

The `check-runner` agent can still run the same checks when a separate context is useful.
