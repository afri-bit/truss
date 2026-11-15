# Agent memory and state

Shared, committed, reviewed memory for every agent and human working on TRUSS. It is
independent of Spec Kit and of any one tool. Decision: [ADR 0001](../docs/decisions/0001-claude-code-agentic-setup.md).

Three parts, three jobs:

| Part | Answers | Format | Lives in |
|---|---|---|---|
| **Memory** | What do we know, and what did we learn? | Markdown + YAML front matter | `memory/MEM-nnnn-slug.md` |
| **State** | Where is this piece of work right now? | JSON, one file per work item | `state/<work-id>.json` |
| **Log** | What happened, in order? | JSON Lines, append-only | `log/YYYY-MM.jsonl` |

## Standards used (and why)

| Standard | Why |
|---|---|
| Markdown with YAML front matter | Readable and diffable on a phone; opens in any editor, Obsidian or Jekyll; no tool lock-in |
| ISO 8601 / RFC 3339 UTC timestamps (`2026-10-30T09:15:00Z`) | Sorts as text, no time zone mistakes, parsed by everything |
| JSON Schema draft 2020-12 (`schema/`) | The contract any tool or language can validate against |
| JSON for current state | One object, easy to read and patch from code |
| JSON Lines for the log | Append-only, one event per line, merges cleanly (`merge=union` in `.gitattributes`) |
| Stable IDs (`MEM-0001`, `ADR-0001`, `specs/003-step-dependencies`) | Cross-links survive renames and moves |
| Semantic / episodic / procedural memory types | The usual split in agent memory: facts, events, how-to |
| Git as history and review | Every change is a diff in a pull request; nothing is silently edited |

No Claude-specific paths or syntax. Any agent or person can read it.

## Memory

One fact per file. Type guides what to write:

| Type | Holds | Body sections |
|---|---|---|
| `semantic` | A stable fact or preference: "commits are authored by afri-bit" | What, Why it matters, How to apply |
| `procedural` | A how-to or lesson: "verify cross-references after parallel workers" | What, Why it matters, How to apply |
| `episodic` | Something that happened: "PR 1 merged" | What happened, Outcome, Lesson |

Front matter (flat keys, lists written inline as `[a, b]`; schema in `schema/memory-entry.schema.json`):

```yaml
---
id: MEM-0001
type: semantic            # semantic | episodic | procedural
title: Short title
summary: One line that is enough to decide whether to read more.
topics: [git, owner-preferences]   # lowercase-kebab, used for search
created: 2026-10-30T09:15:00Z
updated: 2026-10-30T09:15:00Z
author: orchestrator      # agent name, or owner
source: conversation 2026-10-29/30   # PR, session, file, URL
status: active            # active | superseded | archived
confidence: high          # high | medium | low
review_by: 2027-01-31     # optional: re-check by this date
supersedes: MEM-0000      # optional: the entry this replaces
related: [ADR-0001, specs/001-x]   # optional links
---
```

`memory/INDEX.md` is generated. Never edit it by hand; `python3 scripts/memory.py index` rebuilds it, and
`python3 scripts/memory.py validate` fails when it is stale.

### What does not belong in memory

| Put it here instead | When |
|---|---|
| `docs/decisions/` (ADR) | A decision with context and consequences |
| `docs/MILESTONE.md`, `specs/` | Release scope and required behavior |
| Code, tests, git history | Anything that can be derived from them |
| Nowhere | Secrets, credentials, personal data, real hostnames or usernames. The validator rejects them. |

Memory may link to those (`related:`) but must not copy them.

### Not the same as Claude Code's own memory

Claude Code keeps a personal auto-memory outside the repository. That one stays personal and
untracked. This one is shared, committed and reviewed in pull requests, so every agent and the
owner see the same facts.

## State

`state/<work-id>.json` describes one unit of work: tier, branch, status, current step, steps done,
specs, blockers, open questions, next actions, PR. The work id is the branch name
with `/` replaced by `--` (`feature/004-example` becomes
`feature--004-example`). One file per work item means branches never conflict.
Close the work by setting `status=done`; the file stays as history. Schema:
`schema/work-state.schema.json`.

## Log

`log/YYYY-MM.jsonl`: one event per line (`ts`, `actor`, `event`, `work_id`, `summary`, `refs`).
Events: `work_started`, `step_done`, `decision`, `blocker`, `handoff`, `memory_added`,
`memory_updated`, `pr_opened`, `note`. Append only; never edit a past line. Schema:
`schema/event.schema.json`.

## The tool

`scripts/memory.py` uses Python 3 only (standard library; no packages, no network).

```
python3 scripts/memory.py validate      validate everything (also runs in CI)
python3 scripts/memory.py index         rebuild INDEX.md
python3 -m unittest discover -s scripts/tests   run the tool's tests

python3 scripts/memory.py search --topic go [--type T] [--status any] [--text word]
python3 scripts/memory.py new --type semantic --title "..." --summary "..." --topics a,b \
    --author <agent> --source "<where it came from>" --confidence high --body -   # body on stdin
python3 scripts/memory.py update MEM-0001 --confidence high --add-topics docker
python3 scripts/memory.py new ... --supersedes MEM-0001     # replace, never duplicate
python3 scripts/memory.py stale                              # entries past review_by
python3 scripts/memory.py state init <work-id> --title "..." --tier 1 --branch feature/001-x
python3 scripts/memory.py state set <work-id> step=speckit-plan add-step-done=speckit-specify
python3 scripts/memory.py state list --open
python3 scripts/memory.py log --actor planner --event decision --work <work-id> --summary "..." --refs ADR-0004
```

Set `MEMORY_NOW=2026-10-30T09:15:00Z` to fix the clock (tests and seeding). `new` and `update`
rebuild the index. Front matter is changed through the tool; the body prose can be edited by hand
(then run `index` and `validate`).

## Who does what

| Who | Reads | Writes |
|---|---|---|
| Every agent | `INDEX.md` and a topic search at the start of a task | Memory (`/remember`), work state and log at each step |
| `orchestrator` | Open work (`state list --open`) | State for the unit of work, handoffs (`/handoff`) |
| `memory-keeper` | Everything | Reviews, supersedes, archives, rebuilds the index (`/memory-review`) |
| Owner | Everything | Approves memory in pull requests |

Rules for agents: `.claude/rules/agent-memory.md`.
