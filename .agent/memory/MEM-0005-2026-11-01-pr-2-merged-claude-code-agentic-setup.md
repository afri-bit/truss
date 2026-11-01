---
id: MEM-0005
type: episodic
title: 2026-11-01 PR 2 merged: Claude Code agentic setup
summary: The Cursor to Claude Code migration and supervised agentic setup landed on develop.
topics: [agentic-setup, spec-kit]
created: 2026-11-01T09:00:00Z
updated: 2026-11-01T09:00:00Z
author: orchestrator
source: PR #2
status: active
confidence: high
related: [ADR-0001]
---

## What happened

PR #2 moved Spec Kit to the claude integration, removed `.cursor/`, and added agents, skills, hooks, rules, shared memory and a disabled unattended workflow set.

## Outcome

Merged into `develop`. Supervised mode is in force; unattended is a placeholder.

## Lesson

Nothing in the setup ran live before merge. The next real feature is its first test: record what breaks.
