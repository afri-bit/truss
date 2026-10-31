# 0001. Claude Code as the agent platform, with an agentic setup

- Status: Proposed
- Date: 2026-10-30

## Context

TRUSS was built with Spec Kit 0.6.1 for Cursor: the skills lived in `.cursor/skills`, the coding
rules in `.cursor/rules`, and an assistant did one prompt at a time. The maintainer also runs
Senda and Project Justice with Claude Code, where work is delegated to specialised agents under
hooks, rules and shared memory. TRUSS should work the same way, so that an issue can go from
request to a reviewed pull request with the maintainer approving at the end.

## Decision

1. **Switch the Spec Kit integration to `claude`** with the official CLI
   (`specify integration switch claude`, 0.6.1 to 1.2.6.dev0). Customized templates and the
   test-first change to `speckit-tasks` are kept. `.cursor/` is removed.
2. **Orchestrator plus agents.** The main session (`CLAUDE.md`) classifies work into three tiers
   (chore, feature, contract/execution safety), runs the Spec Kit pipeline through `/feature`, and
   delegates to 13 agents in `.claude/agents/` with `low`/`high` model routing. An unattended run
   uses the `lead` agent from the `agent` workflow.
3. **Rules, hooks, permissions.** Cursor rules become path-scoped `.claude/rules/*.md`. Hooks block
   work on `develop`/`main`, force pushes, tag pushes, destructive commands and secret writes, and
   run `gofmt` after edits. `settings.json` allows the Go toolchain and denies merge, release and tag.
4. **Shared memory and state in `.agent/`**, using one tool (`scripts/memory.py`, standard library
   only) and checked in CI. It is independent of Claude Code and Spec Kit.
5. **Supervised first.** A person starts every session and reviews every PR. Unattended mode
   (`lead`, `agent.yml`, `agent-gate.yml`, `scripts/agent_gate.py`) is kept as a disabled placeholder.
   CODEOWNERS marks the guardrail paths.
6. **Agent-to-agent and workspace.** Agents communicate through delegation briefs, handoff reports,
   the work state, the log, memory and ADRs. `.agent/` is the shared board; `/consult`, `/fan-out`
   and `/team-status` operate on it.
7. **Constitution 1.3.0** adds principle VI (Supervised Agentic Delivery), an "Agentic Operation"
   section (modes, team, orchestration, protocol, workspace, autonomy, change control) and a
   quality gate, and drops the obsolete branching tooling note.

## Consequences

- Good: one repeatable, auditable way to build features; the maintainer reviews PRs, not steps.
- Good: mechanical guardrails (hooks, permissions, gate) instead of trust.
- Bad: more files to maintain; the setup must change with the process (same-PR rule).
- Bad: the unattended placeholders can rot; `scripts/agent_gate.py` keeps its tests in CI to slow that.
- Follow-ups: enabling unattended mode needs a constitution amendment and the steps in
  `docs/agents.md`; enable branch protection on `develop` and `main`; add principle VI to the plan
  template's Constitution Check; the templates still show an unprefixed branch placeholder.
