---
id: MEM-0002
type: procedural
title: Switch the Spec Kit integration with the official CLI
summary: Use specify integration switch; it keeps customized templates, then commit the untouched result before adapting.
topics: [spec-kit, tooling]
created: 2026-10-30T23:00:00Z
updated: 2026-10-30T23:00:00Z
author: orchestrator
source: chore(speckit) switch to claude, 2026-10-30
status: active
confidence: high
review_by: 2027-04-30
---

## What

`uvx --from git+https://github.com/github/spec-kit.git specify integration switch claude --script sh` replaced the cursor-agent integration (0.6.1) with the claude one (1.2.6.dev0). It removes the old skills, installs `.claude/skills/speckit-*`, and preserves customized templates in `.specify/templates/`.

## Why it matters

A pristine switch commit makes every later adaptation a visible diff from upstream and eases the next upgrade.

## How to apply

Run it on a clean branch and commit it alone. Then re-apply local customizations in separate commits (see the speckit-tasks entry). Never undo a half-done switch with `git checkout -- .`; it reverts tracked files and loses the work.
