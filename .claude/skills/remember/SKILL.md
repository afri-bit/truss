---
name: "remember"
description: "Save a durable fact, preference, lesson or outcome to the shared agent memory (.agent/memory). Use when the maintainer states a preference or decision not yet in the docs, or after a non-obvious lesson."
argument-hint: "What to remember, e.g. 'prefer table-driven tests for the pipeline parser'"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Remember

Read `.claude/rules/agent-memory.md` first. Memory is shared and committed, so keep it small and true.

1. Decide if it belongs: it must not be a secret, personal data or consultation content; not derivable
   from code or git; not already in the constitution, a spec, an ADR or the milestone
   (link those with `related:` instead). If it fails, say why and stop.
2. Search for duplicates: `python3 scripts/memory.py search --text "<keyword>" --status any`.
   If an entry covers it, update it (`update`) or replace it (`new ... --supersedes MEM-nnnn`).
3. Pick the type: `semantic` (fact or preference), `procedural` (how-to or lesson), `episodic`
   (something that happened).
4. Create it with the tool, body on stdin using the sections for that type:
   `python3 scripts/memory.py new --type ... --title ... --summary ... --topics a,b --author <your agent name> --source "<where this came from>" --confidence <high|medium|low> [--review-by YYYY-MM-DD] [--related ADR-0001] --body -`
5. Log it: `python3 scripts/memory.py log --actor <agent> --event memory_added --work <work-id> --summary "..." --refs MEM-nnnn`.
6. Run `python3 scripts/memory.py validate`. Report the new ID and one-line summary. Do not commit.
