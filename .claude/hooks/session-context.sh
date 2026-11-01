#!/usr/bin/env bash
# SessionStart hook: give the session its bearings. Output goes into the context.
root=${CLAUDE_PROJECT_DIR:-$(pwd)}
cd "$root" || exit 0

branch=$(git branch --show-current 2>/dev/null || true)
dirty=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
echo "TRUSS session context"
echo "- Branch: ${branch:-detached}; uncommitted files: ${dirty:-0}"
case "$branch" in
  main | master | develop) echo "- WARNING: you are on '$branch'. Branch from develop as feature/, bugfix/ or hotfix/ before changing anything." ;;
esac
if [ -f .specify/feature.json ] && command -v jq >/dev/null 2>&1; then
  feature=$(jq -r '.feature_directory // empty' .specify/feature.json 2>/dev/null)
  [ -n "$feature" ] && echo "- Active spec: $feature"
fi
echo "- Pipeline: behavioral changes follow /feature (specify, clarify, plan, tasks, analyze, implement); read docs/MILESTONE.md first."
if command -v python3 >/dev/null 2>&1 && [ -f scripts/memory.py ]; then
  python3 scripts/memory.py summary 2>/dev/null || echo "- Memory: unreadable, run: python3 scripts/memory.py validate"
fi
echo "- Read AGENTS.md and CLAUDE.md; roster, flow and limits are in docs/agents.md."
exit 0
