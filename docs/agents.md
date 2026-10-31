# Agent setup

How Claude Code builds TRUSS with a team of specialised agents. The decision is in
[`decisions/0001-claude-code-agentic-setup.md`](decisions/0001-claude-code-agentic-setup.md).
The principles are in the [constitution](../.specify/memory/constitution.md); the workflow is in
[`AGENTS.md`](../AGENTS.md); orchestration rules are in [`CLAUDE.md`](../CLAUDE.md).

## Layers

| Layer | Location | Purpose |
|---|---|---|
| Principles | `.specify/memory/constitution.md`, `docs/MILESTONE.md` | What is non-negotiable; what the release contains |
| Specs | `specs/NNN-name/`, `.specify/` | Spec Kit artifacts and infrastructure (integration: `claude`) |
| Coding rules | `.claude/rules/*.md` | Go, architecture, TDD, execution safety, contracts; loaded by file path |
| Agents | `.claude/agents/*.md` | Focused roles with their own tools and model |
| Skills | `.claude/skills/*/SKILL.md` | Repeatable commands (`/feature`, `/review`, `/speckit-*`, ...) |
| Guardrails | `.claude/settings.json`, `.claude/hooks/`, `.github/CODEOWNERS` | Permissions and mechanical safety |
| Memory and state | `.agent/`, `scripts/memory.py` | Facts, lessons, work state, event log |
| Enforcement | `.github/workflows/`, `scripts/agent_gate.py` | CI checks and the agent gate |

## Roster

The main session is the **orchestrator** (model alias `opusplan`). Agents do not call agents.

| Agent | Model | Access | Use for |
|---|---|---|---|
| `lead` | opus | edit + delegate | The orchestrator of an unattended run (main session only) |
| `planner` | opus | read-only | Approach, boundaries, compatibility, task list tagged `low`/`high` |
| `architect` | opus | docs | ADRs, constitution amendments, dependency and boundary decisions |
| `go-engineer` | sonnet | edit | Go code in `cmd/` and `internal/`, test-first |
| `test-writer` | sonnet | edit | Failing tests first; contract, integration and e2e tests |
| `devops-engineer` | sonnet | edit | Workflows, release scripts, lint configuration |
| `docs-writer` | haiku | docs | README, CHANGELOG, formatting ADRs, PR text |
| `reviewer` | opus | read-only | Independent code review |
| `spec-reviewer` | opus | read-only | Implementation versus spec, plan, tasks and constitution gates |
| `security-reviewer` | opus | read-only | Execution safety, secrets, input handling, supply chain |
| `portability-reviewer` | sonnet | read-only | Linux, macOS and Windows behavior |
| `check-runner` | haiku | read-only | Runs the gate and reports compactly |
| `memory-keeper` | haiku | `.agent/` | Reviews, deduplicates and supersedes memory |

## Model policy

| Work | Model |
|---|---|
| Architecture, planning, security, every review (except portability) | opus |
| Go code, tests, workflows, tasks tagged `high` | sonnet |
| Mechanical edits, docs, checks, tasks tagged `low` | haiku |

- The `planner` tags each task `low` or `high`; the orchestrator passes `haiku` or `sonnet` as the
  Agent `model`. An untagged task runs on sonnet.
- Never `low`: process execution, cancellation, secret handling, contract changes, concurrency, a
  new pattern. When unsure, `high`.
- Escalate `low` to sonnet after two failures; sonnet to opus after two failures on a design problem.

## Delivery flow

| Tier | Class | Flow |
|---|---|---|
| 0 | Chore | branch, change, `/verify`, PR. No spec |
| 1 | Feature | `/speckit-specify`, `-clarify`, planner, `-plan`, `-tasks`, `-analyze` gate, commit specs, `-implement` (test first), `/verify`, `/review`, docs, PR |
| 2 | Contract or execution safety | Tier 1 plus clarify always, planner always, compatibility note, ADR where decided, security and portability review, "Needs a human" list |

```
REQUEST ─ /recall ─ classify (tier) ─ read MILESTONE
SPEC      /speckit-specify → /speckit-clarify → planner → /speckit-plan → /speckit-tasks
GATE      /speckit-analyze: no CRITICAL or HIGH · commit + push specs/NNN-name/
BUILD     /speckit-implement: test-writer (red) → go-engineer (green) → tick task
QUALITY   /verify → /review (reviewer, spec-, security-, portability-reviewer) → fix → /verify
SHIP      docs-writer → /handoff → /git-commit → /pr-description → PR against develop (never merge)
```

Tools enforce at every tier: hooks, permissions, CI and the agent gate.

## Quality gates

| Gate | Where | Fails when |
|---|---|---|
| Format and vet | CI, `/verify` | `gofmt -l .` prints anything, `go vet` complains |
| Lint | CI, `/verify` | `golangci-lint` (includes `gosec`) |
| Tests | CI on 3 OSes, `/verify` | any failure; skipped or weakened tests are rejected in review |
| End to end | `.github/scripts/e2e-test.sh` | built-binary behavior differs |
| Memory | CI `agent-tooling` | invalid memory, state or log; stale index; secret-like content |
| Spec analysis | `/speckit-analyze` | CRITICAL or HIGH finding |
| Agent gate | CI `agent-gate` | branch name, missing spec artifacts, code before specs, missing work state, guardrail paths touched, missing `Open tasks:` line |
| Independent review | `/review` | open blocker from any reviewer |
| Human | pull request | the maintainer alone merges |

## Handoff report (every agent ends with this)

```
Done:        <what changed, files or packages>
Checks:      <commands run and result>
Decisions:   <anything an ADR should record>
Needs human: <questions, assumptions, contract or scope items>
Follow-ups:  <out of scope things noticed>
```

## Autonomy

| Level | Actions |
|---|---|
| **Free** | Read, edit, format, test; create branches; commit; push feature branches; open PRs against `develop`; write ADRs and docs; update memory and state |
| **Ask the maintainer** | A feature outside `docs/MILESTONE.md`; a materially ambiguous requirement; a new third-party module; an incompatible contract change; a constitution amendment; anything that costs money, needs credentials or releases |
| **Never** | Merge; tag or release; force-push; rewrite pushed history; commit or push to `develop`/`main`; read or write `.env` or key files; skip hooks; weaken, skip or delete a failing test or check |

Hooks are a safety net for agent sessions, not a security boundary. GitHub branch protection on
`develop` and `main`, required checks and CODEOWNERS are the real gate.

## Hooks

| Hook | Event | Effect |
|---|---|---|
| `guard-git.sh` | PreToolUse Bash | Blocks commits/pushes on `develop`/`main`, pushes to them, force pushes, tag pushes, `--no-verify`, remote deletes, new branches without `feature/`, `bugfix/` or `hotfix/` |
| `guard-destructive.sh` | PreToolUse Bash | Blocks `rm -rf` outside build dirs, `git reset --hard`, `git clean`, `gh pr merge`, `gh release`, `sudo`, `curl \| sh`, reading `.env` and key files |
| `guard-secrets.sh` | PreToolUse Write/Edit | Blocks private keys, well-known credential formats and `.env` files |
| `format-on-edit.sh` | PostToolUse Write/Edit | Runs `gofmt` on edited `.go` files |
| `session-context.sh` | SessionStart | Prints branch, active spec, open work and memory status |

## Memory and state

Details: [`.agent/README.md`](../.agent/README.md) and `.claude/rules/agent-memory.md`.

| Part | Format | Use |
|---|---|---|
| Memory `.agent/memory/MEM-nnnn-*.md` | Markdown + YAML front matter | Durable facts, preferences, lessons, outcomes |
| Work state `.agent/state/<work-id>.json` | JSON, one file per branch | Tier, step, steps done, blockers, next actions, PR |
| Event log `.agent/log/YYYY-MM.jsonl` | JSON Lines, append only | Decisions, handoffs, PRs |

`/recall` at the start of a task, `/remember` for a lasting fact, `/handoff` at the end,
`/memory-review` for cleanup. Memory ships in the PR that caused it and never holds secrets or
repeats the constitution or a spec.

## Skills

| Skill | Purpose |
|---|---|
| `/feature` | The delivery pipeline for one change |
| `/speckit-*` | Spec Kit: specify, clarify, plan, tasks, analyze, implement, converge, checklist, constitution, taskstoissues, git helpers |
| `/verify` | Quality gate via `check-runner` |
| `/review` | Independent parallel review |
| `/adr` | Numbered decision record |
| `/git-commit` | Clustered conventional commits, no attribution |
| `/pr-description` | Short phone-readable PR text |
| `/recall`, `/remember`, `/handoff`, `/memory-review` | Shared memory and state |

## Turning on the unattended agent

`.github/workflows/agent.yml` builds an issue when a maintainer adds the `agent:build` label. It is
inert until the maintainer does these one-time steps (none can be done by an agent):

1. Add the repository secret `ANTHROPIC_API_KEY` and install the Claude GitHub App.
2. Add a ruleset on `develop` (and `main`) that requires a PR, code owner review, and the checks
   `agent-gate` and the CI `Test` jobs. The workflow refuses to run without it.
3. Optionally set the repository variable `AGENT_LOGINS` to the account the agent opens PRs as.
4. Pin the third-party actions in the workflows to full commit SHAs before the secret is added.
5. Create the labels `agent:build`, `agent:done`, `agent:blocked` and `chore`.

## Maintaining the setup

- Change a rule, agent or skill in the same PR as the behavior it governs.
- After any Spec Kit upgrade, check that `speckit-tasks` still says "Tests are REQUIRED".
- Add an agent only when a recurring job needs its own tools, model or point of view.
