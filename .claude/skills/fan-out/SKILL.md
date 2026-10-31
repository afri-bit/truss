---
name: "fan-out"
description: "Run independent tasks of one plan in parallel with separate agents and branches or worktrees, then merge, cross-check and verify. Use when tasks.md has several tasks marked [P] that touch different files or packages."
argument-hint: "Task IDs to run in parallel, e.g. 'T004 T005 T006'"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Fan out

1. Read `specs/NNN-name/tasks.md`. Take the tasks named in the input (or all unticked `[P]` tasks of
   the current phase). Refuse tasks that depend on each other, share a file, or touch execution safety
   or a public contract without a finished design: those run one at a time.
2. Check the independence rule: no two tasks edit the same file or package. If they do, serialize them.
3. For each task, write a delegation brief (goal, spec path, files in scope, rules, expected output,
   what not to touch, the failing test to write first). Pick the agent and model from the task tag
   (`low` haiku, `high` sonnet).
4. Spawn all agents **in one turn**. For tasks that could collide, give each its own worktree
   (`isolation: worktree`) and merge afterwards.
5. When all report: read each handoff report, merge worktrees, and check cross-references yourself
   (symbols, imports, docs and tests that refer to another worker's change). Workers cannot see each
   other's edits.
6. Run `/verify`. On failure apply the escalation ladder per task (retry with the error, stronger
   model, `/consult`, then the maintainer).
7. Tick the finished tasks in `tasks.md`, update the work state (`state set ... add-step-done=...`) and
   `/remember` a lesson if a collision or surprise happened. Do not commit unless the pipeline says so.
