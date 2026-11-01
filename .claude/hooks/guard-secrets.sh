#!/usr/bin/env bash
# PreToolUse hook for Write, Edit and MultiEdit: keep secrets out of the repository.
#
# Blocks (exit 2):
#   - writing .env files (except .env.example), *.pem, *.key, SSH keys, credentials.json
#   - content containing private keys or well-known credential formats (cloud, GitHub, Slack,
#     Stripe-style, Anthropic, Google)
#
# Reads the hook JSON from stdin. Exit 0 allows the write.
set -eo pipefail

input=$(cat)
file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
content=$(printf '%s' "$input" | jq -r '[.tool_input.content, .tool_input.new_string, (.tool_input.edits[]?.new_string)] | map(select(. != null)) | join("\n")')

deny() {
  echo "Blocked by .claude/hooks/guard-secrets.sh: $1" >&2
  exit 2
}

base=${file##*/}
case "$base" in
  .env.example | *.example) ;;
  .env | .env.* | *.pem | *.key | id_rsa* | id_ed25519* | credentials.json)
    deny "do not write secret files ($base). Document the variable in .env.example instead." ;;
esac

[ -z "$content" ] && exit 0

patterns=(
  '-----BEGIN [A-Z ]*PRIVATE KEY-----'
  'AKIA[0-9A-Z]{16}'
  'ghp_[A-Za-z0-9]{36}'
  'github_pat_[A-Za-z0-9_]{22,}'
  'xox[baprs]-[A-Za-z0-9-]{10,}'
  'sk-ant-[A-Za-z0-9_-]{20,}'
  '[rs]k_live_[A-Za-z0-9]{16,}'
  'AIza[0-9A-Za-z_-]{35}'
)
for p in "${patterns[@]}"; do
  if printf '%s' "$content" | grep -Eq -- "$p"; then
    deny "the content looks like a credential or private key. Use an environment variable and .env.example."
  fi
done

exit 0
