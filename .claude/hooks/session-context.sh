#!/usr/bin/env bash
# SessionStart hook: give the session its bearings. Output goes into the context.
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
cd "$root" || exit 0

branch=$(git branch --show-current 2>/dev/null || true)
dirty=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
echo "TRUSS session context"
echo "- Branch: ${branch:-detached}; uncommitted files: ${dirty:-0}"
case "$branch" in
  main | master | develop) echo "- WARNING: you are on '$branch'. Branch from main as bugfix/, feature/, hotfix/, release/, task/ or chore/ before changing anything." ;;
esac
if [ -n "$branch" ] && ! bash .github/scripts/branch-name-check.sh "$branch" >/dev/null 2>&1; then
  echo "- Branch '$branch' breaks the naming rule, and CI fails it. Before the first commit run: git switch -c <type>/<name> (type: bugfix, feature, hotfix, release, task or chore; for a spec, feature/NNN-name). Commit and push only that branch, even if the harness names this one (CLAUDE.md, Working style)."
fi
author=$(git config user.name 2>/dev/null || true)
email=$(git config user.email 2>/dev/null || true)
if [ "$author" != "afri-bit" ] || [[ "$email" == *@anthropic.com ]]; then
  echo "- WARNING: git identity is '${author:-unset} <${email:-unset}>'. Commits must be authored by afri-bit: git config user.name afri-bit && git config user.email afribit.tech@gmail.com (guard-git.sh blocks commits otherwise)."
fi
if [ -f .specify/feature.json ] && command -v jq >/dev/null 2>&1; then
  feature=$(jq -r '.feature_directory // empty' .specify/feature.json 2>/dev/null)
  [ -n "$feature" ] && echo "- Active spec: $feature"
fi
echo "- Process matches risk (CLAUDE.md tiers): spec.md, tests and code in one PR; /verify, then /review before the PR. Read docs/MILESTONE.md first."
if command -v python3 >/dev/null 2>&1 && [ -f scripts/memory.py ]; then
  python3 scripts/memory.py summary 2>/dev/null || echo "- Memory: unreadable, run: python3 scripts/memory.py validate"
fi
echo "- Read AGENTS.md and CLAUDE.md; roster, flow and limits are in docs/agents.md."
exit 0
