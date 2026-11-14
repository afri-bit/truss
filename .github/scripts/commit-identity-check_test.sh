#!/usr/bin/env bash
# Tests for commit-identity-check.sh. Run: bash .github/scripts/commit-identity-check_test.sh
set -u

script="$(cd "$(dirname "$0")" && pwd)/commit-identity-check.sh"
failures=0
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cd "$work" || exit 1
git init -q -b main .
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null

# commit NAME EMAIL MESSAGE
commit() {
	GIT_AUTHOR_NAME="$1" GIT_AUTHOR_EMAIL="$2" GIT_COMMITTER_NAME="$1" GIT_COMMITTER_EMAIL="$2" \
		git commit -q --allow-empty -m "$3"
}

# check NAME WANT_EXIT WANT_SUBSTRING
check() {
	local name="$1" want_exit="$2" want="$3" out code
	out="$(bash "$script" base HEAD 2>&1)"
	code=$?
	if [ "$code" -ne "$want_exit" ]; then
		echo "FAIL $name: exit = $code, want $want_exit; output: $out"
		failures=$((failures + 1))
	elif [ -n "$want" ] && [[ "$out" != *"$want"* ]]; then
		echo "FAIL $name: output missing \"$want\"; output: $out"
		failures=$((failures + 1))
	else
		echo "ok   $name"
	fi
}

commit someone else@example.com "root"
git tag base

commit afri-bit 20866800+afri-bit@users.noreply.github.com "one"
check "project identity passes" 0 "1 commit(s)"

commit afri-bit afri-bit@users.noreply.github.com $'two\n\nCo-authored-by: Someone Else <else@example.com>'
check "co-author trailer is allowed" 0 "2 commit(s)"

commit Claude noreply@anthropic.com "three"
check "tool author fails" 1 "authored by \"Claude\""

git reset -q --hard HEAD~1
commit afri-bit noreply@anthropic.com "four"
check "tool email fails" 1 "tool address"

git reset -q --hard HEAD~1
commit Afrizal 20866800+afri-bit@users.noreply.github.com "five"
check "other author name fails" 1 "must be \"afri-bit\""

git reset -q --hard HEAD~1
git checkout -q -b side base
commit afri-bit a@b.c "side"
git checkout -q main
GIT_AUTHOR_NAME=Claude GIT_AUTHOR_EMAIL=noreply@anthropic.com GIT_COMMITTER_NAME=GitHub GIT_COMMITTER_EMAIL=noreply@github.com \
	git merge -q --no-ff side -m "Merge pull request" >/dev/null 2>&1
check "merge commits are skipped" 0 ""

out="$(bash "$script" 2>&1)"; code=$?
[ "$code" -eq 2 ] && echo "ok   usage error" || { echo "FAIL usage error: $code $out"; failures=$((failures + 1)); }

if [ "$failures" -gt 0 ]; then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
