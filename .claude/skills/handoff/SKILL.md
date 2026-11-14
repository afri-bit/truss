---
name: "handoff"
description: "Close out a unit of work or save context before it is lost: update the work state, log the handoff, write an episodic memory, and produce the handoff report. Use at the end of a task, before a long pause, or before context is compacted."
argument-hint: "Optional work id (default: the current branch)"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Handoff

The PR description is the record of where a piece of work stands: keep its "Needs a human", checks
and follow-ups current. A work-state file is optional; keep one only when the work continues in a later
session before a PR exists, or spans several PRs.

1. If a PR exists, update its description. If the work needs a state file, use the work id from the
   input, else `python3 scripts/memory.py state id "$(git branch --show-current)"`, and create it with
   `state init` if missing.
2. If there is a state file, update it to the real situation: `state set <work-id> step=... status=...
   add-blocker=... add-question=... add-next=... pr=<url>`. Remove resolved items. Use `status=done`
   once merged or abandoned; stale open items are flagged at session start.
3. Log a decision or handoff that is not in the PR or an ADR:
   `python3 scripts/memory.py log --actor <agent> --event handoff --work <work-id> --summary "..." --refs <IDs>`.
4. If something happened that a future agent should know (outcome, failure, lesson), write an episodic or
   procedural entry with `/remember`. Do not duplicate what is in an ADR or the PR.
5. Run `python3 scripts/memory.py validate`.
6. Output the handoff report from `docs/agents.md`: Done, Requirements, Checks, Decisions, Needs human,
   Follow-ups. Do not commit unless asked.
