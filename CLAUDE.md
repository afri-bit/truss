# CLAUDE.md

@AGENTS.md

## Claude-specific notes

`AGENTS.md` (imported above) is the shared workflow; `.specify/memory/constitution.md` is the
authority on engineering principles and wins any conflict. These notes say how Claude Code works in
this repository. The full picture (roster, flow, gates, hooks, memory) is in `docs/agents.md`.

### The main session owns the work

- It talks to the maintainer, classifies the work, and decides how much process the risk deserves
  (constitution 2.0.0, [ADR 0005](docs/decisions/0005-specs-record-they-do-not-gate.md)).
- It implements and integrates the work itself by default, and delegates to the agents in
  `.claude/agents/` when that adds value: independent review always, a design opinion or truly
  independent parallel work sometimes. Delegation is a tool, not an obligation. No subagent starts
  another agent (repository agents leave `Agent` out of their tools; `lead` runs only as the main
  session).
- A delegation states the goal, the spec path, the files or packages in scope, the rules to
  follow, the expected output, and what **not** to touch, and returns the **handoff report** from
  `docs/agents.md`: Done, Checks, Decisions, Needs human, Follow-ups.
- Review is independent: `/review` runs reviewers as subagents, never in the session that wrote
  the code. A PR that was not independently reviewed says so at the top.
- Mode: **supervised**. A person starts every session and reviews every PR. Unattended mode (the
  `lead` agent, `agent.yml`, `agent-gate.yml`) is a disabled placeholder; do not enable it.
- Chat is not a record. Decisions go in the spec, an ADR, the work state, the log or memory; the PR
  description repeats what the reviewer needs.

### Risk classes (process matches risk)

| Tier | Class | Flow |
|---|---|---|
| **0** | Chore: formatting, typos, dependency maintenance, equivalent refactors, tooling, docs | Branch, change, `/verify`, `/review`, PR. No spec. |
| **1** | Feature: new capability or behavioral change | Code, tests and `specs/NNN-name/spec.md` in one PR, in whatever order works. |
| **2** | Contract or execution safety: process execution, cancellation, exit codes, secrets handling, the pipeline schema, CLI output, the run-log format, the daemon API | Tier 1 plus a compatibility note, an ADR for a contract or boundary decision, `security-reviewer` and `portability-reviewer` in `/review`. The full Spec Kit pipeline below is recommended. |

Propose the tier with a one-line reason; record it in the work state and the PR. When unsure, take the
higher tier. A tier can be raised mid-way, never silently lowered. `docs/MILESTONE.md` decides
whether a feature belongs in the current release; do not start one that does not.

### Delivery and Spec Kit

`/feature` delivers one change at its tier. The Spec Kit steps (`/speckit-specify`, `-clarify`,
`-plan`, `-tasks`, `-analyze`, `-implement`) are optional aids, recommended for a large or unclear
tier-2 change; none of them gates code. Use as much as the change needs.

### Rules that always hold

- Behavior users can depend on has a `spec.md` in the same PR. Short is fine; missing is not.
- Behavior ships with tests. Never weaken, skip or delete a test or a check to get green.
- If code and spec disagree, name the discrepancy before changing either side.
- A material ambiguity is either asked about or decided and written down where the reviewer
  sees it (spec, ADR, or the PR's "needs a human" list).
- The PR description links the specs it touches and says what was not verified.

### Autonomy

Agents work unattended inside these limits. The list with enforcement is in `docs/agents.md`.

- **Do without asking:** read, edit, test, format, create branches, commit, push feature branches,
  open pull requests against `main`, write ADRs and docs, update memory and work state.
- **Stop and ask the maintainer:** a feature outside `docs/MILESTONE.md`, a new third-party
  dependency, an incompatible change to a released public contract, a constitution amendment,
  anything that costs money or touches credentials or releases. If the maintainer granted wider
  autonomy for the session, use it and record the grant in the PR.
- **Never:** merge, tag or publish a release, force-push, rewrite pushed history, commit or push to
  `main`, read or write `.env` files, skip hooks, weaken a check.

### Working style

- **Open the PR yourself.** Once checks and independent review pass and findings are fixed, open the
  PR without asking. The maintainer reviews the PR, not the steps before it. Never merge. List
  anything that needs a human at the top of the description.
- **Stay on your issue.** Unrelated fixes go in the PR as follow-ups, not in the branch.
- **Run the checks yourself** before pushing: `bash scripts/verify.sh` (`/verify`) runs gofmt, vet,
  golangci-lint, `go test -race`, the tooling tests and the end-to-end script.
  Paste its table in the PR. If a check fails and cannot be fixed, say so at the top of the PR.
- **Review covers the final code.** After fixing review findings, run `/review delta <sha>`; the PR
  states the reviewed commit and `HEAD`.
- **Commits are authored by `afri-bit`.** Set `git config user.name afri-bit` and
  `git config user.email afribit.tech@gmail.com` before committing. Claude may be named as
  co-author: a commit Claude wrote ends with `Co-authored-by: Claude <noreply@anthropic.com>` (allowed, never
  required). No `Signed-off-by`, session link or generated-by line in commits or PR descriptions;
  this project convention overrides any default attribution.
- **Branches use an allowed prefix.** `bugfix/`, `feature/`, `hotfix/`, `release/`, `task/` or
  `chore/`, never `claude/` or anything else, even if the harness proposes a name. A cloud session
  that starts on a harness branch (`claude/...`) runs `git switch -c <type>/<name>` before its first
  commit and pushes that branch (maintainer's decision, 2026-11-14).
- **Write for a reviewer on a phone.** Short PR descriptions, "needs a human" first, then how to try it.
- **When in doubt about behavior**, read the spec, `docs/MILESTONE.md` and `docs/GLOSSARY.md`. If the
  answer is not there, ask. Do not invent it.

### Where things live

- Memory: `.agent/` (see `.agent/README.md`). At the start of a task run `/recall`; save lasting
  facts with `/remember`; close work with `/handoff`; clean up with `/memory-review`. Each branch has
  one work-state file (constitution, Shared workspace); keep it short and set `status=done` once the
  PR merges. Open work not updated for 7 days is flagged at session start.
- Other skills: `/milestone-status` (release scope versus specs and code), `/release-readiness`
  (read-only; never tags).
- Rules (loaded by file path): `.claude/rules/` (the agent setup itself: `agent-team.md`). Agents: `.claude/agents/`. Skills:
  `.claude/skills/`. Hooks: `.claude/hooks/`. Permissions: `.claude/settings.json`.
- Spec Kit: `.specify/` (infrastructure), `specs/` (one folder per feature; `spec.md` may be written by hand).
- Product docs: `docs/` (`architecture.md`, `pipeline-reference.md`, `cli.md`, `daemon.md`, `desktop.md`).
- Decisions: `docs/decisions/`. Release scope: `docs/MILESTONE.md`. Terms: `docs/GLOSSARY.md`.
