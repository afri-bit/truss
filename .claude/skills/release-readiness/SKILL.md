---
name: "release-readiness"
description: "Check whether TRUSS can be tagged: milestone items done, go.mod version, CHANGELOG section, CI green, release-check script. Read-only; never tags or publishes. Use before the maintainer cuts a release."
argument-hint: "Version to check, e.g. 0.1.0"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Release readiness

Read-only. Tagging and publishing are the maintainer's (constitution, Autonomy limits).

1. Version: the input, else the `// version:` line in `go.mod`.
2. Run `/milestone-status`; every "In this release" item must be implemented.
3. `go.mod` version equals the target; `CHANGELOG.md` has a `## [X.Y.Z] - YYYY-MM-DD` section as its
   latest, with entries (see `.github/scripts/release-check.sh`; run
   `bash .github/scripts/release-check_test.sh`).
4. `/verify` (gofmt, vet, lint, race tests, e2e). CI status of `main`: `gh run list --branch main`.
5. No open work in `python3 scripts/memory.py state list --open` that blocks the release.
6. Report ready or not ready with the blocking items first. Never create a tag, release or push.
