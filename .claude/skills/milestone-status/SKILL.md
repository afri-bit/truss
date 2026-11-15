---
name: "milestone-status"
description: "Compare docs/MILESTONE.md with the specs, tasks and code: which release items are specified, planned, implemented, or still missing, and what the next spec should be. Use before choosing the next feature or when asked how far the release is."
argument-hint: "Optional release, default: the current one"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Milestone status

Read-only. Delegate the file reading to `Explore` or `spec-reviewer` if it is large.

1. Read `docs/MILESTONE.md`: the "In this release" items, the "Already specified" table, "Not in" list.
2. For every `specs/NNN-name/`, read the first lines of `spec.md` (status) and count ticked versus open
   tasks in `tasks.md`.
3. Map each "In this release" item to a spec and an implementation (search `internal/` and `cmd/`).
   Mark each: implemented, specified but open tasks, no spec yet, or out of scope.
4. Report a short table, the gap list, and the next spec to write. Say plainly when an item has no
   spec; do not start one. A feature outside the milestone is the maintainer's decision.
5. Note any drift: a spec marked implemented with open tasks, or code with no spec.
