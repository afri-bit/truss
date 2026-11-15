#!/usr/bin/env bash
# Usage: branch-name-check.sh <branch>
#
# Exits 0 when <branch> follows the repository's branch naming rule
# (constitution, Branching), 1 when it does not, 2 on a usage error.
# main and develop are the long-lived branches and are always accepted.
# Must stay compatible with bash 3.2 (macOS).
set -u

if [ $# -ne 1 ] || [ -z "$1" ]; then
	echo "usage: branch-name-check.sh <branch>" >&2
	exit 2
fi

branch="$1"
case "$branch" in
main | develop) exit 0 ;;
esac

re='^(bugfix|feature|hotfix|release|task|chore)/[a-z0-9][a-z0-9._-]*$'
if [[ "$branch" =~ $re ]]; then
	exit 0
fi

echo "error: branch '$branch' is not allowed. Use <type>/<name> with <type> one of bugfix, feature, hotfix, release, task, chore, and <name> lowercase, hyphenated (letters, digits, '.', '_', '-')." >&2
exit 1
