#!/usr/bin/env bash
# Tests for verify.sh: it passes clean code, fails a broken check, never
# reports a missing tool or a wrong lint version as a pass, and rejects check
# names it does not know. Uses a throwaway module, so it is fast.
# Needs go and gofmt on PATH. Run: bash scripts/verify_test.sh
set -u

root="$(cd "$(dirname "$0")/.." && pwd)"
for t in go gofmt git; do
	command -v "$t" >/dev/null 2>&1 || { echo "verify_test: $t not found on PATH; cannot run" >&2; exit 1; }
done
failures=0
work="$(mktemp -d)" || exit 1
[ -n "$work" ] && [ -d "$work" ] || exit 1
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/repo/scripts" "$work/bin" "$work/gobin" "$work/fakelint" "$work/gopath"
cp "$root/scripts/verify.sh" "$work/repo/scripts/verify.sh"
printf 'module example.com/v\n\ngo 1.21\n' >"$work/repo/go.mod"
printf 'package v\n\n// V is a value.\nconst V = 1\n' >"$work/repo/v.go"
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
git -C "$work/repo" init -q
git -C "$work/repo" add -A
git -C "$work/repo" -c user.name=t -c user.email=t@example.com commit -q -m fixture

# A PATH with the basic tools; go in its own directory; no golangci-lint.
for t in bash dirname mktemp rm head sed grep cat git tr; do
	ln -s "$(command -v "$t")" "$work/bin/$t"
done
for t in go gofmt; do ln -s "$(command -v "$t")" "$work/gobin/$t"; done
# A golangci-lint of the wrong version.
printf '#!/bin/sh\necho "golangci-lint has version 2.5.0 built with go1.25.1"\n' >"$work/fakelint/golangci-lint"
chmod +x "$work/fakelint/golangci-lint"

# expect NAME WANT_EXIT WANT_LINE PATH ARGS...
expect() {
	local name="$1" want="$2" line="$3" path="$4"
	shift 4
	out="$(cd "$work/repo" && PATH="$path" GOENV=off GOPATH="$work/gopath" GOBIN="" GOTOOLCHAIN=local GOFLAGS=-mod=mod \
		bash scripts/verify.sh "$@" 2>&1)"
	code=$?
	if [ "$code" -ne "$want" ]; then
		echo "FAIL $name: exit = $code, want $want"
		echo "$out"
		failures=$((failures + 1))
	elif ! printf '%s\n' "$out" | grep -qE "$line"; then
		echo "FAIL $name: output lacks /$line/"
		echo "$out"
		failures=$((failures + 1))
	fi
}

ok="$work/bin:$work/gobin"
expect "formatted code passes" 0 '^gofmt +pass' "$ok" --only gofmt
expect "the result names the commit" 0 '^commit +[0-9a-f]{7}' "$ok" --only gofmt
expect "a pass says PASS" 0 '^result +PASS' "$ok" --only gofmt

printf 'package v\nconst  W=2\n' >"$work/repo/w.go"
expect "unformatted code fails" 1 '^gofmt +fail' "$ok" --only gofmt
expect "a failure says FAIL" 1 '^result +FAIL' "$ok" --only gofmt
rm "$work/repo/w.go"

expect "missing lint is not a pass" 1 '^lint +missing' "$ok" --only lint
expect "wrong lint version is not a pass" 1 '^lint +fail .*not v2\.15\.0' "$work/fakelint:$ok" --only lint
expect "missing go is not a pass" 1 '^vet +missing' "$work/bin" --only vet
expect "an unknown check name is a usage error" 2 "unknown check 'lnt'" "$ok" --only lnt
expect "an empty --only is a usage error" 2 'usage' "$ok" --only ''
expect "an --only without names is a usage error" 2 'usage' "$ok" --only ,
expect "an unknown option is a usage error" 2 'usage' "$ok" --bogus

if [ "$failures" -ne 0 ]; then
	echo "verify_test: $failures failure(s)"
	exit 1
fi
echo "verify_test: ok"
