---
name: "review"
description: "Run an independent multi-agent review of the current branch: general reviewer and spec-reviewer, plus security and portability reviewers when relevant. Records the commit reviewed; 'delta' reviews only the changes since the last review. Use before opening or updating a PR."
argument-hint: "Optional: a focus such as 'cancellation', a PR number, or 'delta <sha>' to review only changes since <sha>"
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

## Full review (default)

1. Scope: `git diff develop...HEAD --name-only` and `git log develop..HEAD --oneline`. If there is
   no diff, say so and stop. Note the commit under review: `git rev-parse --short HEAD`. Commit
   or stash first, so the commit is what the reviewers read.
2. Always spawn `reviewer` (diff, spec/plan path if any, focus from the input, the commit).
3. In parallel, spawn:
   - `spec-reviewer` if the branch has a `specs/NNN-name/` folder or changes behavior.
   - `security-reviewer` if the diff touches `internal/executor`, `internal/engine`,
     `internal/runlog`, `internal/pipeline` (input handling), `cmd/`, `go.mod`, `go.sum`, `.github/`,
     `.claude/`, `scripts/` or any environment, secret or log handling.
   - `portability-reviewer` if the diff touches process execution, paths, shells, signals, the CLI,
     `.github/scripts/` or the CI workflows.
4. Merge the reports into one list ordered by severity (blockers, should fix, nits). Remove
   duplicates, keep `file:line` references, and state each reviewer's verdict **verbatim**. Start the
   result with `Reviewed: <commit>` and the reviewers that ran.
5. Do not apply fixes unless asked. Offer the next step: fix, then `/review delta <commit>`.

## Delta review (`delta <sha>`)

Use it after fixing findings, or after any code change once a full review has run, so the review
covers the code that is merged. It is cheap: reviewers read only what changed since `<sha>`.

1. Changes: `git log --no-merges --oneline <sha>..HEAD` and `git diff <sha> HEAD`. If `develop` was
   merged in since `<sha>`, leave out what that merge brought in (it was reviewed in its own PR) and
   keep only this branch's commits and any conflict resolution (`git show --remerge-diff <merge>`).
2. If only docs or `.agent/` changed, say so: no re-review needed. Otherwise spawn the reviewers whose
   findings were fixed, plus any that the changed paths now trigger (rules above). Give each the
   earlier findings and ask: were they fixed, and do the changes introduce anything new?
3. Report as in step 4 above, starting with `Reviewed: <sha>..<HEAD>`.

## In the pull request

Under "Checks", state `Reviewed: <commit>` and `HEAD: <commit>`. They are equal, or a delta review
covers the difference. If they differ, list under "Needs a human" what was not reviewed and why.
