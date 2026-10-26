#!/usr/bin/env bash
# Tests for release-check.sh. Run: bash .github/scripts/release-check_test.sh
set -u

script="$(cd "$(dirname "$0")" && pwd)/release-check.sh"
failures=0
work=""

setup() {
	work="$(mktemp -d)"
	cat >"$work/go.mod" <<'EOF'
// version: 1.2.0
module github.com/afri-bit/truss

go 1.22.5
EOF
	cat >"$work/CHANGELOG.md" <<'EOF'
# Changelog

## [Unreleased]

## [1.2.0] - 2026-10-10

### Added

- Something new.

## [1.1.0] - 2026-09-01

### Fixed

- Something old.
EOF
}

teardown() {
	rm -rf "$work"
}

# check NAME WANT_EXIT WANT_SUBSTRING TAG DATE
check() {
	local name="$1" want_exit="$2" want="$3" tag="$4" date="$5"
	local out code
	out="$(cd "$work" && bash "$script" "$tag" "$date" "$work/notes.md" 2>&1)"
	code=$?
	if [ "$code" -ne "$want_exit" ]; then
		echo "FAIL $name: exit = $code, want $want_exit; output: $out"
		failures=$((failures + 1))
		return
	fi
	if [ -n "$want" ] && [[ "$out" != *"$want"* ]]; then
		echo "FAIL $name: output missing \"$want\"; output: $out"
		failures=$((failures + 1))
		return
	fi
	echo "ok   $name"
}

setup
check "valid release" 0 "" v1.2.0 2026-10-10
if ! grep -q "Something new." "$work/notes.md" || grep -q "Something old." "$work/notes.md" || grep -q "## \[" "$work/notes.md"; then
	echo "FAIL notes contain only the released section"
	failures=$((failures + 1))
else
	echo "ok   notes contain only the released section"
fi
teardown

for tag in 1.2.0 v1.2 v01.2.0 v1.2.0+build.1 v1.2.0- vX.Y.Z; do
	setup
	check "invalid tag $tag" 1 "not a valid semantic version tag" "$tag" 2026-10-10
	teardown
done

setup
sed -i.bak 's/^## \[1.2.0\]/## [1.2.0-rc.1]/' "$work/CHANGELOG.md"
sed -i.bak 's#^// version: 1.2.0#// version: 1.2.0-rc.1#' "$work/go.mod"
check "pre-release tag" 0 "" v1.2.0-rc.1 2026-10-10
teardown

setup
check "go.mod version differs" 1 "go.mod version 1.2.0 does not match tag v1.2.1" v1.2.1 2026-10-10
teardown

setup
sed -i.bak '/^\/\/ version:/d' "$work/go.mod"
check "go.mod version missing" 1 "go.mod has no \"// version: X.Y.Z\" line" v1.2.0 2026-10-10
teardown

setup
printf '// version: 2.0.0\nmodule github.com/afri-bit/truss\n\ngo 1.22.5\n' >"$work/go.mod"
sed -i.bak 's/^## \[1.2.0\]/## [2.0.0]/' "$work/CHANGELOG.md"
check "v2 without /v2 module path" 1 "module path must end in /v2" v2.0.0 2026-10-10
teardown

setup
printf '// version: 2.0.0\nmodule github.com/afri-bit/truss/v2\n\ngo 1.22.5\n' >"$work/go.mod"
sed -i.bak 's/^## \[1.2.0\]/## [2.0.0]/' "$work/CHANGELOG.md"
check "v2 with /v2 module path" 0 "" v2.0.0 2026-10-10
teardown

setup
sed -i.bak 's#^module github.com/afri-bit/truss#module github.com/afri-bit/truss/v2#' "$work/go.mod"
check "v1 with /v2 module path" 1 "module path must not have a major version suffix" v1.2.0 2026-10-10
teardown

setup
printf '// version: 1.3.0\nmodule github.com/afri-bit/truss\n\ngo 1.22.5\n' >"$work/go.mod"
check "changelog section missing" 1 "CHANGELOG.md has no section for 1.3.0" v1.3.0 2026-10-10
teardown

setup
printf '// version: 1.1.0\nmodule github.com/afri-bit/truss\n\ngo 1.22.5\n' >"$work/go.mod"
check "changelog section not latest" 1 "1.1.0 is not the latest section in CHANGELOG.md (latest is 1.2.0)" v1.1.0 2026-09-01
teardown

setup
check "changelog date differs" 1 "date 2026-10-10 does not match tag date 2026-10-11" v1.2.0 2026-10-11
teardown

setup
sed -i.bak 's/^## \[1.2.0\] - 2026-10-10/## [1.2.0]/' "$work/CHANGELOG.md"
check "changelog date missing" 1 "CHANGELOG.md section for 1.2.0 has no date" v1.2.0 2026-10-10
teardown

setup
sed -i.bak 's/^- Something new\.$//' "$work/CHANGELOG.md"
check "changelog section empty" 1 "CHANGELOG.md section for 1.2.0 has no entries" v1.2.0 2026-10-10
teardown

setup
check "missing arguments" 2 "usage:" "" ""
teardown

if [ "$failures" -gt 0 ]; then
	echo "$failures test(s) failed"
	exit 1
fi
echo "all release-check tests passed"
