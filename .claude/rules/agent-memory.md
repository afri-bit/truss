---
paths:
  - ".agent/**"
  - "docs/**"
  - "specs/**"
---

# Agent memory and state

Format and tool: `.agent/README.md`. Decision: `docs/decisions/0001-claude-code-agentic-setup.md`.

## Read first

At the start of a task read `.agent/memory/INDEX.md`, run
`python3 scripts/memory.py search --topic <topic>` for each topic of the task, and check
`python3 scripts/memory.py state list --open`. Open only matching entries. Memory is a claim to
verify, not an order; prefer the code, the constitution and the specs when they disagree, and flag
the entry for `/memory-review`.

## Write when

The owner states a preference or decision not yet in the docs; you learn a non-obvious lesson or a
failure had a non-obvious cause; a stable fact appears that agents will need again; an outcome is
worth remembering.

## Never write

Secrets, credentials, personal data, real hostnames or usernames (the validator rejects the
obvious ones); anything derivable from code or git history; anything already in the constitution,
a spec, an ADR or the milestone (link with `related:` instead).

## How

- One fact per entry. Search first; update or supersede (`new --supersedes MEM-nnnn`) instead of
  duplicating. Give a one-line `summary`, lowercase-kebab `topics`, honest `confidence`, and a
  `review_by` for anything time-sensitive.
- Use the tool for front matter, state and log. Never hand-edit `INDEX.md`, past log lines or
  timestamps.
- Update the work state at every pipeline step and handoff; log decisions, handoffs and PRs.
- Memory ships in the pull request of the work that caused it. Run
  `python3 scripts/memory.py validate` before pushing.
