---
name: "handoff"
description: "Close out a unit of work or save context before it is lost: update the work state (and the PR description if there is one), log the handoff, write an episodic memory, and produce the handoff report. Use at the end of a task, before a long pause, or before context is compacted."
argument-hint: "Optional work id (default: the current branch)"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Handoff

1. Work id: the input, else `python3 scripts/memory.py state id "$(git branch --show-current)"`. If no
   state exists, create it with `state init` (risk class and tier from `CLAUDE.md`).
2. Update the state to the real situation, briefly: `state set <work-id> step=... status=...
   add-blocker=... add-question=... add-next=... pr=<url>`. Remove resolved items. Use `status=done`
   once the PR is merged or the work abandoned; open items not updated for 7 days are flagged at
   session start. If a PR exists, keep its "Needs a human", checks and follow-ups current too.
3. Log a decision or handoff that is not in the PR or an ADR:
   `python3 scripts/memory.py log --actor <agent> --event handoff --work <work-id> --summary "..." --refs <IDs>`.
4. If something happened that a future agent should know (outcome, failure, lesson), write an episodic or
   procedural entry with `/remember`. Do not duplicate what is in an ADR or the PR.
5. Run `python3 scripts/memory.py validate`.
6. Output the handoff report from `docs/agents.md`: Done, Requirements, Checks, Decisions, Needs human,
   Follow-ups. Do not commit unless asked.
