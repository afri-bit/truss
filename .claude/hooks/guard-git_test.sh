#!/usr/bin/env bash
# Tests for guard-git.sh: a failing and a passing example for each rule it enforces here.
# Run: bash .claude/hooks/guard-git_test.sh
set -u

root="$(cd "$(dirname "$0")/../.." && pwd)"
hook="$root/.claude/hooks/guard-git.sh"
failures=0
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
mkdir -p "$work/.github"
cp -R "$root/.github/scripts" "$work/.github/scripts"
git -C "$work" init -q -b feature/test
git -C "$work" config user.name afri-bit
git -C "$work" config user.email 20866800+afri-bit@users.noreply.github.com

# run COMMAND -> exit code of the hook
run() {
	jq -n --arg c "$1" '{tool_input: {command: $c}}' | CLAUDE_PROJECT_DIR="$work" bash "$hook" >/dev/null 2>&1
}

# check NAME WANT_EXIT COMMAND
check() {
	run "$3"
	local code=$?
	if [ "$code" -ne "$2" ]; then
		echo "FAIL $1: exit = $code, want $2 ($3)"
		failures=$((failures + 1))
	else
		echo "ok   $1"
	fi
}

for prefix in feature bugfix hotfix release task chore; do
	check "$prefix/ branch allowed" 0 "git checkout -b $prefix/some-name"
done
check "switch -c allowed" 0 "git switch -c task/x"
check "branch allowed" 0 "git branch chore/x"
check "claude/ branch denied" 2 "git checkout -b claude/some-session"
check "claude/ switch denied" 2 "git switch -c claude/x"
check "claude/ branch denied (git branch)" 2 "git branch claude/x"
check "fix/ prefix denied" 2 "git checkout -b fix/x"
check "no prefix denied" 2 "git checkout -b my-branch"
check "uppercase denied" 2 "git checkout -b feature/Upper"
check "deleting a claude/ branch is not creating one" 0 "git branch -D claude/x"

check "commit by afri-bit allowed" 0 "git commit -m 'feat: x'"
check "co-author trailer allowed" 0 "git commit -m 'feat: x' -m 'Co-authored-by: A B <a@b.c>'"
check "claude as co-author allowed" 0 "git commit -m 'feat: x' -m 'Co-authored-by: Claude <noreply@anthropic.com>'"
check "commit --author denied" 2 "git commit --author='Claude <noreply@anthropic.com>' -m 'feat: x'"
check "commit --author=... denied" 2 "git commit -m 'feat: x' --author=\"Someone <a@b.c>\""
check "abbreviated --auth denied" 2 "git commit --auth=Zed -m 'feat: x'"
check "author env override denied" 2 "GIT_AUTHOR_NAME=Claude git commit -m 'feat: x'"
check "author email env override denied" 2 "env GIT_AUTHOR_EMAIL=x@anthropic.com git commit -m 'feat: x'"
check "committer env override denied" 2 "GIT_COMMITTER_NAME=Claude git commit -m 'feat: x'"
check "git -c user.name denied" 2 "git -c user.name=Claude commit -m 'feat: x'"
check "git -c user.email denied" 2 "git -c user.email=x@anthropic.com commit -m 'feat: x'"
check "git -c other setting allowed" 0 "git -c core.quotepath=off commit -m 'feat: x'"
git -C "$work" config user.name Claude
check "commit as another name denied" 2 "git commit -m 'feat: x'"
git -C "$work" config user.name afri-bit
git -C "$work" config user.email noreply@anthropic.com
check "commit with tool email denied" 2 "git commit -m 'feat: x'"
git -C "$work" config user.email 20866800+afri-bit@users.noreply.github.com

check "push of a feature branch allowed" 0 "git push -u origin feature/test"
check "push to develop denied" 2 "git push origin HEAD:develop"
check "commit --no-verify denied" 2 "git commit --no-verify -m 'x'"

if [ "$failures" -gt 0 ]; then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
