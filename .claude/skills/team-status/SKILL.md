---
name: "team-status"
description: "Show the shared workspace at a glance: open work items, blockers, questions, next actions, recent events, and stale memory. Use at the start of a session, when switching tasks, or before handing work to another session."
argument-hint: "Optional work id to focus on"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Team status

Read-only. Do not edit anything.

1. `python3 scripts/memory.py state list --open`. For each open item read `.agent/state/<work-id>.json`
   (or only the one named in the input).
2. Show a table: work id, branch, tier, status, step, blockers, open questions, next action, PR.
3. Tail the current month's `.agent/log/*.jsonl` (last 10 events) for what happened last.
4. `python3 scripts/memory.py stale` for memory past `review_by`.
5. `git branch -vv` and `git status --short` for branches that have no state file or uncommitted work.
6. Call out: blocked items, items with no recent event, two items on the same branch or files, and
   anything that needs the maintainer. Suggest the next action. Keep it under 25 lines.
