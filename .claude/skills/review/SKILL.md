---
name: "review"
description: "Run an independent multi-agent review of the current branch: general reviewer and spec-reviewer, plus security and portability reviewers when relevant. Use before opening or updating a PR."
argument-hint: "Optional focus, e.g. 'cancellation' or a PR number"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Review

The code under review must not be reviewed by the session that wrote it, so always delegate to
subagents. Do not review the diff yourself.

1. Scope: `git diff develop...HEAD --name-only` and `git log develop..HEAD --oneline`. If there is
   no diff, say so and stop.
2. Always spawn `reviewer` (diff, spec/plan path if any, focus from the input).
3. In parallel, spawn:
   - `spec-reviewer` if the branch has a `specs/NNN-name/` folder or changes behavior.
   - `security-reviewer` if the diff touches `internal/executor`, `internal/engine`,
     `internal/runlog`, `internal/pipeline` (input handling), `cmd/`, `go.mod`, `go.sum`, `.github/`,
     `.claude/`, `scripts/` or any environment, secret or log handling.
   - `portability-reviewer` if the diff touches process execution, paths, shells, signals, the CLI,
     `.github/scripts/` or the CI workflows.
4. Merge the reports into one list ordered by severity (blockers, should fix, nits). Remove
   duplicates, keep `file:line` references, and state each reviewer's verdict.
5. Do not apply fixes unless asked. Offer the next step (fix blockers, then re-run `/review`).
