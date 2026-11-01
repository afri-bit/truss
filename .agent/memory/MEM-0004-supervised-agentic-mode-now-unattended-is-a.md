---
id: MEM-0004
type: semantic
title: Supervised agentic mode now; unattended is a disabled placeholder
summary: The maintainer wants supervised agentic work now; keep the unattended workflows and lead agent as not-running placeholders.
topics: [agents, owner-preferences, workflow]
created: 2026-10-31T00:30:00Z
updated: 2026-10-31T00:30:00Z
author: orchestrator
source: maintainer instruction 2026-10-31
status: active
confidence: high
related: [ADR-0001]
---

## What

TRUSS runs in supervised mode: a person starts every agent session and reviews every PR. `agent.yml`, `agent-gate.yml`, `scripts/agent_gate.py` and the `lead` agent are kept but disabled.

## Why it matters

Enabling unattended mode needs credentials, rulesets and a constitution amendment that the maintainer has not made.

## How to apply

Do not enable the placeholders, and do not delete them. Build the rest of the setup (team, orchestration, agent-to-agent, workspace, rules, skills) as supervised.
