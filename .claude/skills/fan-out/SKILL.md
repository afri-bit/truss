---
name: "fan-out"
description: "Run genuinely independent units of work in parallel with separate agents in worktrees, then merge, cross-check and verify. Use when two or more units share no files and have a fixed interface; most changes do not qualify."
argument-hint: "The units to run in parallel: task IDs from tasks.md, or a short list"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Fan out

Parallel work pays only when units are truly independent; otherwise one session is faster and safer.

1. List the units: the input, `[P]` tasks in `specs/NNN-name/tasks.md` if one exists, or units you
   split from the change yourself. Write the interface between them down first (types, file names,
   API shapes); it does not change while workers run.
2. Independence check: no two units edit the same file, and none touches execution safety or a public
   contract without a finished design. Anything else runs one at a time in this session.
3. For each unit, a delegation brief: goal, spec path, files in scope, the interface, rules, the
   failing test to write first, expected output, what not to touch. Pick the agent (`go-engineer`,
   `test-writer`, `devops-engineer`, or a general agent) and pass a `model` only if the unit is clearly
   mechanical (haiku) or hard (opus).
4. Spawn all workers **in one turn**, each with `isolation: worktree`.
5. When all report: read each handoff report, merge the worktrees one at a time, and check
   cross-references yourself (symbols, imports, docs and tests that refer to another worker's change).
   Workers cannot see each other's edits.
6. `/verify` after the merge. On failure: retry the unit with the full error, then a stronger model,
   then `/consult`, then the maintainer.
7. Tick tasks in `tasks.md` if there is one. Note in the PR that the work was fanned out and how.
   `/remember` a lesson if a collision or surprise happened.

Separate features that need not land in one PR are better as separate cloud sessions and PRs.
