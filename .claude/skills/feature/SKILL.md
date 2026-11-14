---
name: "feature"
description: "Guided end-to-end delivery of one change, from request to open pull request, at the right risk class (chore, feature, or contract/execution-safety) defined in CLAUDE.md. Use when the maintainer asks to build, add, fix or implement something."
argument-hint: "Feature idea or issue, e.g. 'concurrency for ready steps'"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Delivery flow

You are the orchestrator. Classify the work first, then run that class's steps **in order**. Never
skip or reorder a step; say so when a step has nothing to report. Delegate with the Agent tool: state
goal, spec path, scope, rules and expected output, and collect each agent's handoff report
(`docs/agents.md`). Independent tasks go out in parallel in one turn.

Stop and ask the maintainer only for genuine product unknowns, scope outside `docs/MILESTONE.md`,
spend, credentials, or a decision the constitution reserves to a human.

## Preconditions

- Not on `develop`, `main` or `master`. Branch from `develop` as `bugfix/`, `feature/`, `hotfix/`, `release/`, `task/` or `chore/` (never `claude/`)
  (`.claude/rules/git-workflow.md`). If the input is empty, ask for the change in one sentence.
- `/recall` for the topic. Read `docs/MILESTONE.md` and the constitution if not read this session.
- If the request is outside the current release, stop and report it; do not start a spec.

## Step 0: classify and record

Propose a class with a one-line reason; the maintainer can override. Record it:
`python3 scripts/memory.py state init <work-id> --title "..." --tier <0|1|2> --branch <branch>`.

| Tier | Class | For |
|---|---|---|
| 0 | Chore | Formatting, typos, dependency maintenance, equivalent refactors, tooling, docs; a bugfix whose spec already exists |
| 1 | Feature | New capability or behavioral change |
| 2 | Contract / execution safety | A feature or fix that touches process execution, cancellation, exit codes, secrets handling, the pipeline schema, CLI output, or the run-log format |

When unsure take the higher tier. A tier may be raised mid-way (say why, record it), never silently lowered.

## Tier 0

1. Branch. 2. `planner` only if the approach is unclear. 3. Change with `go-engineer` or the right
agent; if observable behavior would change, stop: it is a feature. 4. `/verify`. 5. `docs-writer` if
docs change, `/git-commit`, `/pr-description`, open the PR against `develop`.

## Tier 1 and 2

| # | Step | Who (model) | Output / gate |
|---|---|---|---|
| 1 | `/speckit-specify` | orchestrator | `specs/NNN-name/spec.md`, branch `feature/NNN-name`; user stories with independent tests |
| 2 | `/speckit-clarify`: **always** at tier 2; at tier 1 run it too and let it record "no open questions" if so | orchestrator | Questions answered or the note recorded |
| 3 | Approach: `planner` (**always** at tier 2); `architect` + `/adr` where a contract, boundary or dependency decision is made | `planner`, `architect` (opus) | Boundaries, compatibility note, task tags `low`/`high` |
| 4 | `/speckit-plan`, `/speckit-tasks` | orchestrator | `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`, `tasks.md` (tests ordered first per story) |
| 5 | `/speckit-analyze` | orchestrator | **Gate:** no CRITICAL or HIGH finding; fix the artifacts and re-run (max 3 rounds) |
| 6 | Commit with `/git-commit` and push `specs/NNN-name/` | orchestrator | No code under `cmd/` or `internal/` before steps 1 to 5 are committed |
| 7 | `/speckit-implement`: per task, `test-writer` writes the failing test, then `go-engineer` implements (`low` on haiku, `high` on sonnet), tick `tasks.md` | `test-writer`, `go-engineer` (sonnet) | Red then green, evidence kept; `devops-engineer` for `.github/` tasks |
| 8 | `/verify`, `/review` (tier 2: security and portability reviewers required), fix, `/verify` | `check-runner` (haiku), reviewers (opus) | **Gate:** checks green, no open blocker |
| 9 | `docs-writer` (README, CHANGELOG `[Unreleased]`, ADR), `/handoff`, `/git-commit`, `/pr-description`, open the PR against `develop` | `docs-writer` (haiku) | PR links `specs/NNN-name/`, states open tasks; tier 2: "Needs a human" first |

Spec Kit hooks (`.specify/extensions.yml`): run the mandatory ones. Decline the optional
`speckit-git-commit` auto-commit prompts and commit with `/git-commit` instead. Branch names carry the
constitution's prefix: create it with `GIT_BRANCH_NAME=feature/NNN-name` (NNN is the next free number in
`specs/`). On a fresh checkout set `SPECIFY_FEATURE_DIRECTORY=specs/NNN-name`
(`.claude/rules/specs-and-sdd.md`).

## After each step

Update the work state (`state set <work-id> step=... add-step-done=...`) and, if a step surfaced a
lesson or a fact the next session needs, `/remember` it. Log decisions with `log --event decision`.

## Rules

- Never hand-write Spec Kit artifacts and never rebuild them after code exists. Scope change: update
  the spec and re-run plan, tasks and analyze.
- Test-first, always: no production behavior before a failing test. Never weaken or skip a test.
- Review is independent; never review code in the session that wrote it.
- Never merge, tag or release. Report the PR link and what needs the maintainer.
