---
id: MEM-0008
type: procedural
title: Check ADR numbers and lint toolchain before opening a PR
summary: Parallel branches can take the same ADR number; check origin/develop first. The cloud golangci-lint cannot load this repo's Go 1.27.3 config.
topics: [adr, lint, process]
created: 2026-11-12T22:56:24Z
updated: 2026-11-12T22:56:24Z
author: claude
source: conversation
status: active
confidence: high
related: [ADR-0007, specs/010-run-tracking-and-queue]
---

## What

Before numbering an ADR or spec, fetch and list docs/decisions and specs on origin/develop. In the cloud session golangci-lint is built with Go 1.25 and cannot load the config (module needs Go 1.27.3).

## Why it matters

PR 10 and the secrets PR both took ADR 0006, which forced a renumber to 0007 during the merge. Lint was never run locally.

## How to apply

Run git fetch and ls docs/decisions before writing an ADR. State in the PR that lint did not run and let CI run it.
