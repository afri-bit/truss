#!/usr/bin/env bash
# PreToolUse hook for Bash: block commands that destroy data, infrastructure, or secrets.
#
# Blocks (exit 2):
#   - recursive+force rm, unless every target is a plain build directory (bin, .truss, dist, ...)
#   - git reset --hard, git clean -f, discarding the whole working tree
#   - gh release create|delete|edit and gh pr merge (releasing and merging are the maintainer's)
#   - docker volume/system prune, terraform/kubectl changes, npm|pnpm|yarn publish, sudo
#   - piping a download into a shell (curl ... | sh)
#   - reading .env files or key files through the shell (cat .env, source .env, ...)
#
# A convenience guard for agent sessions, not a security boundary: a shell wrapper such as
# `bash -c "..."` can evade it. Reads the hook JSON from stdin. Exit 0 allows the command.
set -eo pipefail

command=$(jq -r '.tool_input.command // empty')
[ -z "$command" ] && exit 0

deny() {
  echo "Blocked by .claude/hooks/guard-destructive.sh: $1" >&2
  exit 2
}

if printf '%s' "$command" | grep -Eq '(curl|wget)[^|;&]*\|[[:space:]]*(sudo[[:space:]]+)?(ba|z)?sh\b'; then
  deny "do not pipe a download into a shell."
fi

# Drop heredoc bodies and quoted prose so commit messages are not mistaken for commands.
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

segments=$(printf '%s' "$stripped" | tr '\n' ' ' \
  | sed -E "s/\"[^\"]*[[:space:]][^\"]*\"//g; s/'[^']*[[:space:]][^']*'//g" \
  | awk '{ gsub(/&&|\|\||;|\|/, "\n"); print }')

is_secret_file() {
  local f=${1##*/}
  case "$f" in
    .env.example | *.example) return 1 ;;
    .env | .env.* | *.pem | *.key | id_rsa* | id_ed25519* | credentials.json) return 0 ;;
  esac
  return 1
}

is_build_dir() {
  case "${1%/}" in
    bin | .truss | dist | out | coverage | tmp) return 0 ;;
    */bin | */.truss | */dist | */coverage | */tmp) return 0 ;;
  esac
  return 1
}

while IFS= read -r segment; do
  read -ra tok <<<"$segment" || true
  [ "${#tok[@]}" -gt 0 ] || continue
  for i in "${!tok[@]}"; do tok[$i]=${tok[$i]//[\"\']/}; done

  idx=0
  while [[ "${tok[idx]:-}" == *=* ]]; do idx=$((idx + 1)); done
  cmd=${tok[idx]:-}
  args=("${tok[@]:$((idx + 1))}")
  line=" ${tok[*]:$idx} "

  case "$cmd" in
    sudo) deny "do not use sudo." ;;
    rm)
      recursive=0 force=0 targets=()
      for a in "${args[@]}"; do
        case "$a" in
          --recursive) recursive=1 ;;
          --force) force=1 ;;
          --*) ;;
          -*) [[ "$a" == *r* || "$a" == *R* ]] && recursive=1
              [[ "$a" == *f* ]] && force=1 ;;
          *) targets+=("$a") ;;
        esac
      done
      if [ "$recursive" = 1 ] && [ "$force" = 1 ]; then
        for t in "${targets[@]}"; do
          case "$t" in /* | ~* | .. | ../* | */../* | . | ./ | '*') deny "rm -rf outside a build directory ($t)." ;; esac
          is_build_dir "$t" || deny "rm -rf is only allowed on build directories (bin, .truss, dist, ...). Got: $t"
        done
      fi ;;
    git)
      [[ "$line" == *" reset "*"--hard"* ]] && deny "git reset --hard discards work."
      [[ "$line" =~ \ clean\ .*-[a-zA-Z]*f ]] && deny "git clean -f deletes untracked files."
      [[ "$line" == *" checkout -- ."* || "$line" == *" restore ."* ]] && deny "do not discard the whole working tree." ;;
    docker | docker-compose)
      [[ "$line" =~ \ volume\ (rm|prune) ]] && deny "do not remove docker volumes."
      [[ "$line" =~ \ system\ prune ]] && deny "do not prune the docker system." ;;
    terraform | tofu)
      [[ "$line" =~ \ (apply|destroy|import|state) ]] && deny "infrastructure changes need the maintainer." ;;
    kubectl)
      [[ "$line" =~ \ (delete|apply|replace) ]] && deny "cluster changes need the maintainer." ;;
    npm | pnpm | yarn)
      [[ "$line" =~ \ (publish|login|owner|yank) ]] && deny "publishing and registry changes need the maintainer." ;;
    gh)
      [[ "$line" =~ \ pr\ merge ]] && deny "merging is the maintainer's step."
      [[ "$line" =~ \ release\ (create|delete|edit|upload) ]] && deny "releases are published by the release workflow from a tag."
      [[ "$line" =~ \ repo\ (delete|archive|edit) ]] && deny "repository administration needs the maintainer."
      [[ "$line" =~ \ secret\ (set|delete) ]] && deny "secrets are managed by the maintainer." ;;
    cat | less | more | head | tail | grep | egrep | rg | sed | awk | source | . | cp | mv | base64 | strings | xxd | od | bat | nl)
      for a in "${args[@]}"; do
        is_secret_file "$a" && deny "do not read or move secret files ($a). Use .env.example."
      done ;;
  esac
done <<<"$segments"

exit 0
