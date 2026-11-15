---
name: "pr-description"
description: "Draft a short, phone-readable pull request description for the current branch in TRUSS's format (needs-a-human first, then how to try it). Use when opening or updating a PR."
argument-hint: "Optional issue number or notes"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# PR description

1. Gather context: `git log develop..HEAD --oneline`, `git diff develop...HEAD --stat`, and the spec,
   plan, `tasks.md` and issue if known. If there is no diff against `develop`, say so and stop. The
   base branch is `develop`.
2. Check `.github/pull_request_template.md` and mirror its headings.
3. Delegate to `docs-writer` (haiku) to draft it, or write it yourself for a very small change.
4. Keep it short. Use this structure; omit "Needs a human" only when there is nothing for the maintainer:

```markdown
## Needs a human
- <ADRs or constitution amendments to approve, guardrail paths touched (.github/, .claude/, scripts/), contract changes, anything unverified>

## Try it
- Build: `go build -o bin/truss ./cmd/truss`
- Run: <command with a sample truss.yaml>
- Expect: <2-4 short observations>

## What changed
- <concrete change and why>. Spec: `specs/NNN-name/`. Closes #N.

## Checks
- `scripts/verify.sh`: <its table and commit line, or what failed and why>
- Reviewed: <commit the last /review covered> · HEAD: <current commit; a delta review covers any difference>
- Open tasks: <unticked task IDs from tasks.md, or "none">

## Notes
- Dependencies added: <module and why, or "none">
- Compatibility impact: <pipeline schema, CLI, run log, or "none">
- Follow-ups (out of scope): <list, or "none">
```

5. Never invent an issue number, command output or check result; run `/verify` first if results are
   unknown. The project convention is no attribution lines or tool footers: output the markdown only.
