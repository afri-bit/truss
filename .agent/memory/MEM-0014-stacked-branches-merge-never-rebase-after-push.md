---
id: MEM-0014
type: procedural
title: Stacked branches: merge, never rebase after push
summary: Update a pushed stacked branch by merging its base; a rebase needs a force-push, which guard-git.sh blocks. A PR with a non-default base got GitHub 500 twice (2026-11-25); base main plus a note worked.
topics: [git, workflow]
created: 2026-11-25T15:53:41Z
updated: 2026-11-25T15:53:41Z
author: orchestrator
source: conversation
status: active
confidence: medium
---

## What

v0.3.0 was built as stacked branches (024, 025 on 024, 026 on 025, ...). Rebasing a pushed
branch onto its updated base and pushing again needs a force-push, which `guard-git.sh` blocks.

## Why it matters

A rebased branch cannot be pushed; work stalls until the history is reconciled.

## How to apply

- Rebase only branches that were never pushed.
- For a pushed branch, merge the updated base into it (`git merge feature/<base>`).
- If a rebase already happened, `git merge -s ours FETCH_HEAD` of the pushed tip keeps the new
  tree and makes the push a fast-forward.
- Creating a PR with a non-default base failed with HTTP 500 twice; open it against `main` and
  say in the description which PR to merge first.
