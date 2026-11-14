#!/usr/bin/env bash
# PreToolUse hook for Bash: enforce TRUSS's git rules before a command runs.
#
# Blocks (exit 2):
#   - commit, push, merge, rebase, reset, cherry-pick while on develop, main or master
#   - any push that targets develop, main or master (for example `git push origin HEAD:develop`)
#   - force pushes, remote branch deletes, mirror and all-branch pushes
#   - pushing tags (a v* tag publishes a release; that is the maintainer's step)
#   - creating a branch whose name lacks an allowed prefix: bugfix/, feature/, hotfix/, release/,
#     task/ or chore/ (constitution, Branching; the rule lives in .github/scripts/branch-name-check.sh)
#   - committing when the author identity is not afri-bit (git config user.name / user.email)
#   - skipped hooks or signing (--no-verify, --no-gpg-sign, commit -n)
#
# This is a convenience guard for agent sessions, not a security boundary: a shell wrapper
# such as `bash -c "..."` can evade it. The real gate is GitHub branch protection on develop
# and main.
# Reads the hook JSON from stdin. Exit 0 allows the command.
set -eo pipefail

command=$(jq -r '.tool_input.command // empty')
[ -z "$command" ] && exit 0

deny() {
  echo "Blocked by .claude/hooks/guard-git.sh: $1" >&2
  exit 2
}

# True when a push argument names a protected branch (HEAD:develop, +main, refs/heads/main, develop).
is_protected() {
  local ref=${1#+}
  ref=${ref##*:}
  ref=${ref#refs/heads/}
  [ "$ref" = main ] || [ "$ref" = master ] || [ "$ref" = develop ]
}

# True when a new branch name follows the constitution. One rule, shared with CI:
# .github/scripts/branch-name-check.sh. There is no exemption for harness branches (claude/...).
valid_branch_name() {
  bash "${CLAUDE_PROJECT_DIR:-.}/.github/scripts/branch-name-check.sh" "$1" >/dev/null 2>&1
}

BRANCH_RULE="must be <type>/<name> with <type> one of bugfix, feature, hotfix, release, task, chore (constitution, Branching)."

# Drop heredoc bodies so words inside a commit message are not mistaken for options.
stripped=$(printf '%s\n' "$command" | awk -v q="'" '
  skip {
    t = $0; gsub(/^[ \t]+|[ \t]+$/, "", t)
    if (t == delim) skip = 0
    next
  }
  match($0, /<<-?[ \t]*[^ \t)<]+/) && substr($0, RSTART - 1, 1) != "<" {
    d = substr($0, RSTART, RLENGTH)
    gsub(/^<<-?[ \t]*/, "", d); gsub(q, "", d); gsub(/"/, "", d)
    delim = d; skip = 1
    print substr($0, 1, RSTART - 1)
    next
  }
  { print }')

# Drop quoted prose (strings containing whitespace), such as commit messages. A quoted single
# word like "main" is kept, so `git push origin "main"` is still caught.
# Then split into one shell segment per line on ; && || |
segments=$(printf '%s' "$stripped" | tr '\n' ' ' \
  | sed -E "s/\"[^\"]*[[:space:]][^\"]*\"//g; s/'[^']*[[:space:]][^']*'//g" \
  | awk '{ gsub(/&&|\|\||;|\|/, "\n"); print }')

branch=$(git -C "${CLAUDE_PROJECT_DIR:-.}" branch --show-current 2>/dev/null || true)

while IFS= read -r segment; do
  read -ra tok <<<"$segment" || true
  [ "${#tok[@]}" -gt 0 ] || continue
  for i in "${!tok[@]}"; do tok[$i]=${tok[$i]//[\"\']/}; done

  idx=0
  while [[ "${tok[idx]:-}" == *=* ]]; do idx=$((idx + 1)); done
  [ "${tok[idx]:-}" = git ] || continue
  idx=$((idx + 1))
  while [[ "${tok[idx]:-}" == -* ]]; do
    case "${tok[idx]}" in
      -C | -c) idx=$((idx + 2)) ;;
      *) idx=$((idx + 1)) ;;
    esac
  done
  sub=${tok[idx]:-}
  args=("${tok[@]:$((idx + 1))}")

  # Creating a branch: switch -c/-C, checkout -b/-B, branch <name>.
  case "$sub" in
    switch | checkout)
      for ((n = 0; n < ${#args[@]}; n++)); do
        case "${args[n]}" in
          -c | -C | -b | -B)
            name=${args[n + 1]:-}
            [ -n "$name" ] && ! valid_branch_name "$name" && deny "branch '$name' $BRANCH_RULE"
            ;;
        esac
      done
      continue ;;
    branch)
      newname=""
      for a in "${args[@]}"; do
        case "$a" in
          -d | -D | --delete | -m | -M | --move | -c | -C | --copy | --list | -l | -a | -r | -v | -vv | --show-current | --contains | --merged | --no-merged | --set-upstream-to* | -u | --unset-upstream) newname="" ; break ;;
          -*) ;;
          *) [ -z "$newname" ] && newname=$a ;;
        esac
      done
      [ -n "$newname" ] && ! valid_branch_name "$newname" && deny "branch '$newname' $BRANCH_RULE"
      continue ;;
  esac

  case "$sub" in
    commit | push | merge | rebase | reset | cherry-pick) ;;
    *) continue ;;
  esac

  case "$branch" in
    main | master | develop) deny "never change history on '$branch'. Create a branch first (one issue, one branch)." ;;
  esac

  for a in "${args[@]}"; do
    case "$a" in
      --no-verify | --no-gpg-sign) deny "do not skip hooks or signing." ;;
      -n) [ "$sub" = commit ] && deny "do not skip hooks (commit -n)." ;;
    esac
  done

  if [ "$sub" = commit ]; then
    ident=$(git -C "${CLAUDE_PROJECT_DIR:-.}" var GIT_AUTHOR_IDENT 2>/dev/null || true)
    ident_name=${ident%% <*}
    ident_email=${ident#*<}; ident_email=${ident_email%%>*}
    if [ "$ident_name" != "afri-bit" ] || [[ "$ident_email" == *@anthropic.com ]]; then
      deny "commits are authored by afri-bit (got '$ident_name <$ident_email>'). Run: git config user.name afri-bit && git config user.email afribit.tech@gmail.com. Co-authored-by trailers (Claude included) are allowed."
    fi
  fi

  if [ "$sub" = push ]; then
    for a in "${args[@]}"; do
      case "$a" in
        --force | -f | --force-with-lease* | --delete | -d | --mirror | --all)
          deny "do not force-push, delete remote branches, or push all branches." ;;
        --tags | --follow-tags | refs/tags/* | tag | v[0-9]*)
          deny "do not push tags; a v* tag publishes a release and that is the maintainer's step." ;;
        +*) deny "do not force-push (refspec starts with +)." ;;
        :*) deny "do not delete remote branches." ;;
        -*) ;;
        *) is_protected "$a" && deny "never push to develop, main or master. Push a branch and open a pull request." ;;
      esac
    done
  fi
done <<<"$segments"

exit 0
