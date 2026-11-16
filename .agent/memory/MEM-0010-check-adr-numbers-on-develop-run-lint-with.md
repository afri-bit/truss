---
id: MEM-0010
type: procedural
title: Check ADR numbers on main; run lint with scripts/verify.sh
summary: Parallel branches can take the same ADR number; check origin/main first. Lint runs locally through scripts/verify.sh with golangci-lint v2.15.0 built with Go 1.27.3.
topics: [adr, lint, process]
created: 2026-11-14T12:16:48Z
updated: 2026-11-14T12:16:48Z
author: claude
source: PR 21 (chore/verification-gate)
status: active
confidence: high
supersedes: MEM-0008
related: [ADR-0007]
---

## What

Before numbering an ADR or spec, fetch and list `docs/decisions` and `specs` on `origin/main` (`develop` was retired on 2026-11-16).
Lint is a blocking CI check and runs locally with `bash scripts/verify.sh`. In a cloud session the
`truss` environment's setup script installs golangci-lint v2.15.0 built with Go 1.27.3; elsewhere run
`GOTOOLCHAIN=go1.27.3 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`.

## Why it matters

PR 10 and the secrets PR both took ADR 0006, which forced a renumber during the merge. Until PR 21 an
older golangci-lint (built with Go 1.25) could not load the config, so lint never ran locally and was
non-blocking in CI; 25 findings had piled up.

## How to apply

Run `git fetch` and list `docs/decisions` before writing an ADR. Run `scripts/verify.sh` before a PR;
a `missing` or wrong-version lint is a failure to fix, not a line to note in the PR.
