# CLAUDE.md

@AGENTS.md

## Claude-specific notes

`AGENTS.md` (imported above) is the shared workflow; `.specify/memory/constitution.md` is the
authority on engineering principles and wins any conflict. These notes say how Claude Code works in
this repository. The full picture (roster, flow, gates, hooks, memory) is in `docs/agents.md`.

### The main session is the orchestrator

- It talks to the maintainer, classifies the work, plans the sequence, delegates to the agents in
  `.claude/agents/`, and merges their results. It does as little hands-on work as it sensibly can.
- Subagents do not call other subagents. Independent work is delegated in parallel in one turn.
- Every delegation states the goal, the spec path, the files or packages in scope, the rules to
  follow, the expected output, and what **not** to touch.
- Every agent returns the **handoff report** from `docs/agents.md`: Done, Checks, Decisions, Needs
  human, Follow-ups. The orchestrator relays "Needs human" items and puts them at the top of the PR.
- Review is independent: `/review` runs reviewers as subagents, never in the session that wrote
  the code.
- An unattended run (started by the `agent` workflow for one issue) uses the `lead` agent as the
  main session. See `docs/agents.md`.

### Risk classes (process matches risk)

| Tier | Class | Flow |
|---|---|---|
| **0** | Chore: formatting, typos, dependency maintenance, equivalent refactors, tooling, docs | Branch, change, `/verify`, PR. No spec. `planner` only if unclear. If observable behavior would change, it is not a chore. |
| **1** | Feature: new capability or behavioral change | The feature pipeline below. |
| **2** | Contract or execution safety: process execution, cancellation, exit codes, secrets handling, the pipeline schema, CLI output, the run-log format | Tier 1 plus `/speckit-clarify` always, `planner` always, a compatibility note, an ADR where a contract or boundary decision is made, `security-reviewer` and `portability-reviewer` in `/review`, and a "Needs a human" list. |

Propose the tier with a one-line reason and record it in the work state. When unsure, take the
higher tier. A tier can be raised mid-way, never silently lowered. `docs/MILESTONE.md` decides
whether a feature belongs in the current release; do not start one that does not.

### Feature pipeline (tiers 1 and 2, in this order)

Constitution principle I, run through `/feature`. Details and rationale in `docs/agents.md`.

1. `/speckit-specify` creates `specs/NNN-name/spec.md` and the branch `feature/NNN-name`.
2. `/speckit-clarify` runs. If nothing is unclear, the spec records that.
3. `planner` drafts the approach (boundaries, compatibility, task tags `low`/`high`) as input.
4. `/speckit-plan` then `/speckit-tasks` write `plan.md`, `research.md`, `data-model.md`,
   `contracts/`, `quickstart.md` and `tasks.md` (organized by user story, tests first).
5. `/speckit-analyze` is a gate: resolve every CRITICAL and HIGH finding before any code.
6. Commit and push the spec artifacts, then implement with `/speckit-implement`: for each task
   `test-writer` writes the failing test, `go-engineer` makes it pass, and the task is ticked in
   `tasks.md`.
7. `/verify`, `/review`, fix, `/verify`.
8. `docs-writer` (README, `CHANGELOG.md` `[Unreleased]`, ADR), `/handoff`, `/git-commit`,
   `/pr-description`, open the PR against `develop`.

### Pipeline rules

- Run the real skills. Never hand-write `plan.md`, `tasks.md` or other Spec Kit artifacts, and never
  rebuild them after the code exists.
- Never skip or reorder a step within a tier. A step with nothing to report still runs and says so.
- No code under `cmd/` or `internal/` before steps 1 to 5 are committed. A chore may change code
  directly because it changes no behavior.
- If scope changes mid-way, update the spec and re-run plan, tasks and analyze before continuing.
- Test-first (principle III): no production behavior before a failing test. Never weaken, skip or
  delete a test or a check to get green.
- If code and spec disagree, name the discrepancy before changing either side.
- The PR description links `specs/NNN-name/` and states which tasks are open.

### Autonomy

Agents work unattended inside these limits. The list with enforcement is in `docs/agents.md`.

- **Do without asking:** read, edit, test, format, create branches, commit, push feature branches,
  open pull requests against `develop`, write ADRs and docs, update memory and work state.
- **Stop and ask the maintainer:** a feature outside `docs/MILESTONE.md`, a materially ambiguous
  requirement, a new third-party dependency, an incompatible change to a public contract, a
  constitution amendment, anything that costs money or touches credentials or releases.
- **Never:** merge, tag or publish a release, force-push, rewrite pushed history, commit or push to
  `develop` or `main`, read or write `.env` files, skip hooks, weaken a check.

### Working style

- **Open the PR yourself.** Once checks and independent review pass and findings are fixed, open the
  PR without asking. The maintainer reviews the PR, not the steps before it. Never merge. List
  anything that needs a human at the top of the description.
- **Stay on your issue.** Unrelated fixes go in the PR as follow-ups, not in the branch.
- **Run the checks yourself** before pushing (`/verify`): `gofmt -l .`, `go vet ./...`,
  `golangci-lint run ./...`, `go test -race ./...`, and `.github/scripts/e2e-test.sh` when the CLI
  changed. If a check fails and cannot be fixed, say so at the top of the PR.
- **No tool attribution.** Commits and PR descriptions carry no `Co-authored-by`, `Signed-off-by`,
  session link or generated-by line. This project convention overrides any default.
- **Write for a reviewer on a phone.** Short PR descriptions, "needs a human" first, then how to try it.
- **When in doubt about behavior**, read the spec, `docs/MILESTONE.md` and `docs/GLOSSARY.md`. If the
  answer is not there, ask. Do not invent it.

### Where things live

- Memory and state: `.agent/` (see `.agent/README.md`). At the start of a task run `/recall`; save
  lasting facts with `/remember`; close work with `/handoff`; clean up with `/memory-review`.
- Rules (loaded by file path): `.claude/rules/`. Agents: `.claude/agents/`. Skills:
  `.claude/skills/`. Hooks: `.claude/hooks/`. Permissions: `.claude/settings.json`.
- Spec Kit: `.specify/` (infrastructure, do not edit), `specs/` (artifacts, generated only by skills).
- Decisions: `docs/decisions/`. Release scope: `docs/MILESTONE.md`. Terms: `docs/GLOSSARY.md`.
