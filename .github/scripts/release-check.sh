#!/usr/bin/env bash
# Usage: release-check.sh <tag> <tag-date YYYY-MM-DD> [<notes-file>]
#
# Run from the module root. Verifies that <tag> is a semantic version tag
# and that go.mod and CHANGELOG.md declare the same version as their latest
# release, dated <tag-date>. On success, writes that CHANGELOG.md section to
# <notes-file>. Must stay compatible with bash 3.2 (macOS).
set -u

if [ $# -lt 2 ] || [ -z "$1" ] || [ -z "$2" ]; then
	echo "usage: release-check.sh <tag> <tag-date YYYY-MM-DD> [<notes-file>]" >&2
	exit 2
fi

tag="$1"
tag_date="$2"
notes="${3:-}"
errors=0

fail() {
	echo "error: $*" >&2
	errors=$((errors + 1))
}

num='(0|[1-9][0-9]*)'
ident='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
semver="^v${num}\.${num}\.${num}(-${ident}(\.${ident})*)?$"
if ! [[ "$tag" =~ $semver ]]; then
	hint="want vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-PRERELEASE"
	case "$tag" in
	*+*) hint="$hint; Go module versions cannot carry build metadata" ;;
	esac
	fail "$tag is not a valid semantic version tag ($hint)"
	exit 1
fi
version="${tag#v}"
major="${BASH_REMATCH[1]}"

# go.mod: the full version lives in a "// version: X.Y.Z" comment, and the
# module path must follow Go's major version suffix rule.
if [ ! -f go.mod ]; then
	fail "go.mod not found"
else
	count="$(grep -c '^// version:' go.mod)"
	gomod_version="$(sed -n 's#^// version:[[:space:]]*\([^[:space:]]*\)[[:space:]]*$#\1#p' go.mod | head -n 1)"
	if [ "$count" -eq 0 ] || [ -z "$gomod_version" ]; then
		fail "go.mod has no \"// version: X.Y.Z\" line"
	elif [ "$count" -gt 1 ]; then
		fail "go.mod has more than one \"// version:\" line"
	elif [ "$gomod_version" != "$version" ]; then
		fail "go.mod version $gomod_version does not match tag $tag"
	fi

	module="$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod)"
	if [ "$major" -ge 2 ]; then
		case "$module" in
		*/v"$major") ;;
		*) fail "module path must end in /v$major for tag $tag (got $module)" ;;
		esac
	elif [[ "$module" =~ /v[0-9]+$ ]]; then
		fail "module path must not have a major version suffix for tag $tag (got $module)"
	fi
fi

# CHANGELOG.md: the tag's section must be the first released section, carry
# the tag date, and list at least one entry.
if [ ! -f CHANGELOG.md ]; then
	fail "CHANGELOG.md not found"
else
	heading="## [$version]"
	if ! rest="$(awk -v h="$heading" 'index($0, h) == 1 { print substr($0, length(h) + 1); found = 1; exit } END { if (!found) exit 1 }' CHANGELOG.md)"; then
		fail "CHANGELOG.md has no section for $version"
	else
		latest="$(grep -E '^## \[' CHANGELOG.md | grep -v '^## \[Unreleased\]' | head -n 1 | sed 's/^## \[\([^]]*\)\].*/\1/')"
		if [ "$latest" != "$version" ]; then
			fail "$version is not the latest section in CHANGELOG.md (latest is $latest)"
		fi

		date_re='^ - ([0-9]{4}-[0-9]{2}-[0-9]{2})[[:space:]]*$'
		if [[ "$rest" =~ $date_re ]]; then
			if [ "${BASH_REMATCH[1]}" != "$tag_date" ]; then
				fail "CHANGELOG.md date ${BASH_REMATCH[1]} does not match tag date $tag_date for $version"
			fi
		else
			fail "CHANGELOG.md section for $version has no date (want \"## [$version] - $tag_date\")"
		fi

		body="$(awk -v h="$heading" '
			index($0, h) == 1 { in_section = 1; next }
			in_section && /^## / { exit }
			in_section { print }
		' CHANGELOG.md)"
		if ! printf '%s\n' "$body" | grep -Eq '^[[:space:]]*[-*][[:space:]]+[^[:space:]]'; then
			fail "CHANGELOG.md section for $version has no entries"
		fi
	fi
fi

if [ "$errors" -gt 0 ]; then
	exit 1
fi

if [ -n "$notes" ]; then
	printf '%s\n' "$body" | sed -e '/./,$!d' >"$notes"
fi
echo "release $tag is consistent with go.mod and CHANGELOG.md"
