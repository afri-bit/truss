---
name: git-commit
description: Create a git commit with a conventional subject and a bullet-point body, and never add a co-author trailer. Use when the user asks to commit, write a commit message, or says "commit the changes" or "git commit".
---

# Git commit

Commit only when the user asks. Follow the repository's git safety rules: do not change git config, do not skip hooks, do not push, and do not amend a commit that has been pushed or that you did not just create.

## Inspect first

Run these in parallel before drafting the message:

- `git status`
- `git diff` and `git diff --cached`
- `git log -12 --oneline`

Stage only files that belong in the commit. Leave secrets (`.env`, credentials, keys) unstaged and warn if the user asked to include them. Do not create an empty commit.

If the working tree contains unrelated groups of changes, make one commit per group. Otherwise make one commit.

## Message

Use a conventional commit. The subject line always starts with a type:

| Type | Use for |
|---|---|
| `feat` | User-facing behavior |
| `fix` | A defect |
| `docs` | Documentation only |
| `test` | Tests only |
| `refactor` | Structure change with the same behavior |
| `perf` | Performance |
| `build` | Build or dependencies |
| `ci` | CI or release automation |
| `chore` | Maintenance that fits none of the above |
| `style` | Formatting only |
| `revert` | Reverting a commit |

Add a scope when one area is obvious: `ci(release):`, `fix(executor):`.

Subject rules:

- Imperative mood: "add", "fix", "require". Not "added" or "adds".
- One line, about 50–72 characters. Long enough to name the change and where it lands. Short enough to read in `git log --oneline`.
- No trailing period.

Body rules:

- A blank line after the subject, then bullet points of what was done.
- At least two bullets. Usually no more than six.
- Each bullet is one concrete change: what changed and, when it matters, why.
- Do not restate the subject, list every file, or narrate the session.

```
ci: test every change and publish releases from version tags

- Run lint, race tests, and the built binary on Linux, macOS, and Windows.
- Publish a GitHub Release only when the tag, go.mod, and changelog agree.
```

```
fix(executor): keep the step exit code when the shell cannot start

- Return the start error without an exit code when exec fails before the process runs.
- Stop the pipeline on that step so later steps stay unrun.
```

## No co-author trailer

Never append a trailer. This includes `Co-authored-by: Cursor <cursoragent@cursor.com>`, any other `Co-authored-by`, and `Signed-off-by`.

Pass the message only through a HEREDOC:

```bash
git commit -m "$(cat <<'EOF'
type: subject of about fifty to seventy two characters

- First concrete change.
- Second concrete change.

EOF
)"
```

After the commit, read `git log -1 --format=%B`. If a trailer was injected and this commit is still unpushed, remove it with `git commit --amend` and the same HEREDOC. Do not use `--no-verify`. Then run `git status` to confirm the tree is clean or that only intentional leftovers remain.
