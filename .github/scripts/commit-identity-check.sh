#!/usr/bin/env bash
# Usage: commit-identity-check.sh <base> <head>
#
# Every non-merge commit in <base>..<head> must be authored by the project
# identity: author name "afri-bit" and an author email that is not a tool
# address (anything @anthropic.com). Co-authored-by trailers are allowed and
# are not inspected. Run inside the repository. Bash 3.2 compatible.
set -u

if [ $# -ne 2 ] || [ -z "$1" ] || [ -z "$2" ]; then
	echo "usage: commit-identity-check.sh <base> <head>" >&2
	exit 2
fi

want_name="afri-bit"
errors=0
count=0

# %x09 is a tab, so names and emails with spaces survive the split.
if ! list="$(git log --no-merges --format='%H%x09%an%x09%ae' "$1..$2")"; then
	echo "error: cannot list commits in $1..$2" >&2
	exit 2
fi

while IFS="$(printf '\t')" read -r sha name email; do
	[ -n "$sha" ] || continue
	count=$((count + 1))
	short="$(printf '%s' "$sha" | cut -c1-7)"
	if [ "$name" != "$want_name" ]; then
		echo "error: commit $short is authored by \"$name\"; the author name must be \"$want_name\"" >&2
		errors=$((errors + 1))
	fi
	case "$email" in
	*@anthropic.com)
		echo "error: commit $short uses the tool address <$email>; use the $want_name email" >&2
		errors=$((errors + 1))
		;;
	"")
		echo "error: commit $short has no author email" >&2
		errors=$((errors + 1))
		;;
	esac
done <<EOF2
$list
EOF2

if [ "$errors" -gt 0 ]; then
	echo "fix: git config user.name $want_name; git config user.email afribit.tech@gmail.com; then git rebase -r $1 --exec 'git commit --amend --no-edit --reset-author' (only on a branch nobody else has pulled)" >&2
	exit 1
fi
echo "$count commit(s) authored by $want_name"
