#!/usr/bin/env bash
# PostToolUse hook for Write, Edit and MultiEdit: format what was just edited.
# Go files run gofmt (constitution principle II). Never fails the tool call: formatting is a
# convenience, `gofmt -l .` and `golangci-lint` are the gate.
file=$(jq -r '.tool_input.file_path // empty' 2>/dev/null)
[ -n "$file" ] && [ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file" >/dev/null 2>&1 || true ;;
esac
exit 0
