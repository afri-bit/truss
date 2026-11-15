#!/usr/bin/env bash
# Usage: release-check.sh <tag> <tag-date YYYY-MM-DD> [<notes-file>]
#
# Run from the module root. Verifies that <tag> is a semantic version tag
# and that all four declarations of the version agree with it:
#   - go.mod             "// version: X.Y.Z" (the Go module)
#   - cmd/truss          "const Version" in version.go (what `truss version` prints)
#   - TRUSS Desktop      internal/webui/VERSION (kept apart from the Go version)
#   - CHANGELOG.md       the latest released section, dated <tag-date>, using only
#                        Keep a Changelog headings (Added, Changed, Deprecated,
#                        Removed, Fixed, Security)
# On success, writes that CHANGELOG.md section to <notes-file>.
# Must stay compatible with bash 3.2 (macOS).
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

# The Go constant that `truss version` prints.
if [ ! -f cmd/truss/version.go ]; then
	fail "cmd/truss/version.go not found"
else
	const_version="$(sed -n 's#^const Version = "\(.*\)"[[:space:]]*$#\1#p' cmd/truss/version.go | head -n 1)"
	if [ -z "$const_version" ]; then
		fail "cmd/truss/version.go has no 'const Version = \"X.Y.Z\"' line"
	elif [ "$const_version" != "$version" ]; then
		fail "cmd/truss Version $const_version does not match tag $tag"
	fi
fi

# TRUSS Desktop has its own version file, separate from the Go module version.
if [ ! -f internal/webui/VERSION ]; then
	fail "internal/webui/VERSION not found"
else
	desktop_version="$(tr -d '[:space:]' <internal/webui/VERSION)"
	if [ -z "$desktop_version" ]; then
		fail "internal/webui/VERSION is empty"
	elif [ "$desktop_version" != "$version" ]; then
		fail "TRUSS Desktop version $desktop_version (internal/webui/VERSION) does not match tag $tag"
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
		# Only Keep a Changelog headings, so the release notes read the same every time.
		while IFS= read -r line; do
			case "$line" in
			"### Added" | "### Changed" | "### Deprecated" | "### Removed" | "### Fixed" | "### Security") ;;
			"### "*) fail "CHANGELOG.md section for $version has unknown heading \"$line\" (use Added, Changed, Deprecated, Removed, Fixed or Security)" ;;
			esac
		done <<EOF2
$body
EOF2
	fi
fi

if [ "$errors" -gt 0 ]; then
	exit 1
fi

if [ -n "$notes" ]; then
	printf '%s\n' "$body" | sed -e '/./,$!d' >"$notes"
fi
echo "release $tag is consistent with go.mod and CHANGELOG.md"
