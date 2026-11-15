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

You own the change end to end: you implement and integrate it. Process matches risk (constitution
2.0, ADR 0005): `spec.md`, tests and code ship in one PR, in whatever order works; nothing else gates
code. Agents add value where they give an independent view (review always, design sometimes) or real
parallelism. Delegate with a brief (goal, spec path, scope, rules, expected output, what not to touch)
and collect the handoff report (`docs/agents.md`).

Stop and ask the maintainer only for genuine product unknowns, scope outside `docs/MILESTONE.md`, a new
dependency, spend, credentials, or a decision the constitution reserves to a human.

## 0. Start

- Branch from `develop` as `<type>/<name>` (`bugfix/`, `feature/`, `hotfix/`, `release/`, `task/`,
  `chore/`). For a spec, `feature/NNN-name`. On a harness branch (`claude/...`), switch first.
- `/recall` the topic. Read `docs/MILESTONE.md`; if the request is outside the current release, stop
  and report it.
- Classify with a one-line reason and record it:
  `python3 scripts/memory.py state init <work-id> --title "..." --tier <0|1|2> --branch <branch>`
  (one work state per branch; it also goes in the PR). When unsure take the higher tier; never lower
  it silently.

| Tier | Class | For |
|---|---|---|
| 0 | Chore | Formatting, typos, dependency maintenance, equivalent refactors, tooling, docs |
| 1 | Feature | New capability or behavioral change, including a bugfix (start from a failing regression test) |
| 2 | Contract / execution safety | Process execution, cancellation, exit codes, secrets handling, the pipeline schema, CLI output, the run-log format, the daemon API |

## 1. Build

| Tier | Do |
|---|---|
| 0 | Make the change. If observable behavior would change, it is tier 1. |
| 1 | Write or update `specs/NNN-name/spec.md` (short is fine) and the tests with the code. Test first is recommended. |
| 2 | Tier 1, plus: a compatibility note in the spec; `/speckit-clarify` or questions to the maintainer for open product questions; `/consult` (two separate written opinions) and `/adr` for every contract, execution-safety or dependency decision. For a large or unclear change the Spec Kit steps (`/speckit-plan`, `-tasks`, `-analyze`) help; they are optional. |

- Parallel work only for units that share no files and have a fixed interface: `/fan-out`.
- If code and spec disagree, name the discrepancy before changing either side.
- Docs and `CHANGELOG.md` `[Unreleased]` change with the behavior they describe.

## 2. Check

1. `/verify` (`scripts/verify.sh`) until it passes. Never weaken a test or a check.
2. `/review`. Tier 2 includes `security-reviewer` and `portability-reviewer`.
3. Fix the findings, `/verify`, then `/review delta <sha>` so the review covers the final commit.

## 3. Ship

`/git-commit`, push the branch, `/pr-description`, open the PR against `develop`. The PR states the
tier, links the spec, lists "Needs a human" first, pastes the verify table, and gives `Reviewed` and
`HEAD`. `/handoff` updates the work state (`pr=<url>`); `/remember` a lasting lesson.
Never merge, tag or release. Report the PR link and what needs the maintainer.

## Spec Kit notes

When you use Spec Kit: decline the optional `speckit-git-commit` auto-commit prompts and commit with
`/git-commit`. Create the branch with `GIT_BRANCH_NAME=feature/NNN-name` (NNN is the next free number
in `specs/`). On a fresh checkout set `SPECIFY_FEATURE_DIRECTORY=specs/NNN-name`
(`.claude/rules/specs-and-sdd.md`). Do not rebuild Spec Kit artifacts after the code exists; update
`spec.md` instead.
