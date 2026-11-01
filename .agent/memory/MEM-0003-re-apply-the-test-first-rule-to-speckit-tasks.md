---
id: MEM-0003
type: procedural
title: Re-apply the test-first rule to speckit-tasks after a Spec Kit upgrade
summary: The speckit-tasks skill is customized so tests are required; an upgrade can overwrite it.
topics: [spec-kit, testing, tdd]
created: 2026-10-30T23:00:00Z
updated: 2026-10-30T23:00:00Z
author: orchestrator
source: e2d9609 and the cursor to claude migration
status: active
confidence: high
review_by: 2027-04-30
related: [ADR-0001]
---

## What

Upstream `speckit-tasks` says tests are optional. TRUSS changed it to "Tests are REQUIRED" (constitution principle III): test tasks come before implementation tasks per user story. The change lives in `.claude/skills/speckit-tasks/SKILL.md`.

## Why it matters

An upgrade or refresh of the integration silently restores the optional-tests wording, and task lists then stop being test-first.

## How to apply

After any `specify integration upgrade` or `--refresh-shared-infra`, grep the skill for "Tests are REQUIRED". If it is gone, restore it in its own commit.
