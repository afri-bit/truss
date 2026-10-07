---
name: implement-task
description: Implement exactly one task from an approved TRUSS spec's tasks.md, with tests, then tick its checkbox and update spec statuses. Use when the user asks to implement, build, or code a task, task ID (e.g. T004), or feature that has a spec under specs/.
---

# Implement one task

## Steps

1. **Locate** the feature in `specs/` and the task ID. If none was given,
   take the first unchecked task whose dependencies are done and confirm it
   with the user.
2. **Gate**: read the front matter of `spec.md`, `plan.md`, `tasks.md`.
   All three must be `approved` or `implementing`. If any is `draft`,
   **stop** and report; do not implement.
3. **Read** `AGENTS.md`, the constitution, the full spec (especially the FR
   IDs the task references), the plan's Design section, referenced ADRs, and
   the code you will touch.
4. **Check for drift**: if the code or spec contradicts the task, stop and
   report the discrepancy instead of resolving it silently.
5. **Write tests first** for the referenced FRs (table-driven; `testdata/`
   fixtures; `//go:build integration` if Docker is needed). Run them and
   confirm they fail for the right reason.
6. **Implement** the minimum to pass, following `go-conventions` and the
   architecture rules (no forbidden imports, typed events only, `filepath`,
   no shell assumptions). Stay inside the task's files and scope.
7. **Verify**: `make fmt lint test` (and `make test-integration` when
   executors or Docker are touched). Fix everything; never weaken tests.
8. **Update the spec**:
   - tick the task's checkbox in `tasks.md`;
   - if this is the first completed task, set all three files to
     `status: implementing`;
   - if it is the last, confirm every FR has a passing test, then set all
     three to `status: done`;
   - mirror the status in `specs/README.md`.
9. **Docs**: if the CLI surface or schema changed, run the `docs-writer`
   subagent. Add a `CHANGELOG.md` Unreleased entry for user-visible changes.
10. **Review**: run the `code-reviewer` subagent on the diff and address
    findings.
11. **Commit** with a Conventional Commit referencing the task, e.g.
    `feat(pipeline): validate duplicate step ids (T007)`. The PR body links
    the spec directory and lists the task IDs.

## Rules

- One task per change. If the task is too big for one PR, propose splitting
  it in `tasks.md` first.
- Do not implement behavior that is not in the spec, even if it seems
  obviously useful; propose a spec change instead.
