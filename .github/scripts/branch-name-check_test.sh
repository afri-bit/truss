#!/usr/bin/env bash
# Tests for branch-name-check.sh. Run: bash .github/scripts/branch-name-check_test.sh
set -u

script="$(cd "$(dirname "$0")" && pwd)/branch-name-check.sh"
failures=0

# check WANT_EXIT BRANCH
check() {
	local want="$1" branch="$2" code
	bash "$script" "$branch" >/dev/null 2>&1
	code=$?
	if [ "$code" -ne "$want" ]; then
		echo "FAIL '$branch': exit = $code, want $want"
		failures=$((failures + 1))
	else
		echo "ok   '$branch' -> $code"
	fi
}

for ok in main develop feature/003-step-dependencies bugfix/fix-log-race hotfix/exit-code \
	release/0.2.0 task/update-docs chore/bump-go feature/a; do
	check 0 "$ok"
done

for bad in claude/repo-governance-0tceic claude/x feat/x fix/x docs/x feature feature/ feature/Upper \
	feature/-x feature//x feature/a/b Feature/x release task chores/x "feature/with space" \
	origin/feature/x refs/heads/feature/x main/x; do
	check 1 "$bad"
done

check 2 ""
bash "$script" >/dev/null 2>&1
[ $? -eq 2 ] && echo "ok   no argument -> 2" || { echo "FAIL no argument"; failures=$((failures + 1)); }

if [ "$failures" -gt 0 ]; then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
