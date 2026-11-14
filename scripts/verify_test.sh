#!/usr/bin/env bash
# Tests for verify.sh: it passes clean code, fails a broken check, and never
# reports a missing tool as a pass. Uses a throwaway module, so it is fast.
# Run: bash scripts/verify_test.sh
set -u

root="$(cd "$(dirname "$0")/.." && pwd)"
failures=0
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

mkdir -p "$work/repo/scripts" "$work/bin" "$work/gopath"
cp "$root/scripts/verify.sh" "$work/repo/scripts/verify.sh"
printf 'module example.com/v\n\ngo 1.21\n' >"$work/repo/go.mod"
printf 'package v\n\n// V is a value.\nconst V = 1\n' >"$work/repo/v.go"
git -C "$work/repo" init -q
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

# A PATH with the basic tools and go, but no golangci-lint.
for t in bash dirname mktemp rm head sed grep cat git; do
	ln -s "$(command -v "$t")" "$work/bin/$t"
done
gobin="$work/gobin"
mkdir -p "$gobin"
for t in go gofmt; do ln -s "$(command -v "$t")" "$gobin/$t"; done

# expect NAME WANT_EXIT WANT_LINE PATH ARGS...
expect() {
	local name="$1" want="$2" line="$3" path="$4"
	shift 4
	out="$(cd "$work/repo" && PATH="$path" GOPATH="$work/gopath" GOTOOLCHAIN=local GOFLAGS=-mod=mod \
		bash scripts/verify.sh "$@" 2>/dev/null)"
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

expect "formatted code passes" 0 '^gofmt +pass' "$work/bin:$gobin" --only gofmt
expect "pass names the commit and result" 0 '^result +PASS' "$work/bin:$gobin" --only gofmt

printf 'package v\nconst  W=2\n' >"$work/repo/w.go"
expect "unformatted code fails" 1 '^gofmt +fail' "$work/bin:$gobin" --only gofmt
expect "a failure fails the result" 1 '^result +FAIL' "$work/bin:$gobin" --only gofmt
rm "$work/repo/w.go"

expect "missing lint is not a pass" 1 '^lint +missing' "$work/bin:$gobin" --only lint
expect "missing go is not a pass" 1 '^vet +missing' "$work/bin" --only vet
expect "unknown option is a usage error" 2 '' "$work/bin:$gobin" --bogus

if [ "$failures" -ne 0 ]; then
	echo "verify_test: $failures failure(s)"
	exit 1
fi
echo "verify_test: ok"
