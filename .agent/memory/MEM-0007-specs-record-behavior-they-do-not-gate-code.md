---
id: MEM-0007
type: semantic
title: Specs record behavior, they do not gate code
summary: The maintainer wants the SDD process to document features without blocking implementation; constitution 2.0.0 and ADR 0005 make spec.md travel with the code.
topics: [owner-preferences, workflow, spec-kit]
created: 2026-11-12T00:30:00Z
updated: 2026-11-12T00:30:00Z
author: orchestrator
source: maintainer instruction 2026-11-11
status: active
confidence: high
related: [ADR-0005]
---

## What

For the MVP the maintainer asked agents to decide technical questions autonomously, use specs to document features without letting them block implementation, and change the constitution and hard rules where needed. Constitution 2.0.0 records this: `spec.md` and tests ship in the same pull request as the code, in any order.

## Why it matters

The older flow (full Spec Kit sequence and strict test-first before any code) is no longer mandatory. MEM-0003 describes that older rule for the Spec Kit task template and applies only when the full pipeline is used.

## How to apply

Pick the amount of process by risk. Always ship a short `spec.md`, tests, and user docs with a behavioral change, and say in the PR what was not verified or reviewed. Never merge, tag, force-push or weaken a check.
