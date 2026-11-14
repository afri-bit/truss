#!/usr/bin/env bash
# Usage: scripts/verify.sh [--only name,name,...]
#
# TRUSS's local quality gate, the same checks CI runs, for Linux and macOS
# (bash 3.2 or later). Prints one line per check and exits 1 if any check
# fails or its tool is missing, 0 otherwise. A missing tool is reported as
# "missing" and never counts as a pass. Exit 2 is a usage error.
#
# Checks: gofmt, vet, lint (golangci-lint v2.15.0 on Linux, Windows and macOS
# files), test (-race), tooling (memory validation, Python and shell tests),
# e2e (the built binary). --only runs a subset, for example --only gofmt,vet.
#
# The last lines name the commit the result belongs to; paste the output into
# the pull request as verification evidence.
set -u

lint_version=2.15.0
checks="gofmt vet lint test tooling e2e"
usage="usage: verify.sh [--only name,...]   names: ${checks// /, }"

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root" || exit 2

only=""
while [ $# -gt 0 ]; do
	case "$1" in
	--only)
		[ $# -ge 2 ] && [ -n "$2" ] || { echo "$usage" >&2; exit 2; }
		set -f # no glob expansion of the names
		for name in $(printf '%s' "$2" | tr ',' ' '); do
			case " $checks " in *" $name "*) ;; *) echo "verify.sh: unknown check '$name'; $usage" >&2; exit 2 ;; esac
		done
		set +f
		only=",$2,"; shift ;;
	-h | --help) sed -n '2,/^set -u/p' "$0" | sed '$d'; exit 0 ;;
	*) echo "$usage" >&2; exit 2 ;;
	esac
	shift
done

logs="$(mktemp -d)" || exit 2
[ -n "$logs" ] && [ -d "$logs" ] || exit 2
trap 'rm -rf "$logs"' EXIT
failed=0
summary=""

wanted() { [ -z "$only" ] || case "$only" in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

# record NAME RESULT [DETAIL]
record() {
	summary="$summary$(printf '%-9s %s' "$1" "$2")${3:+  ($3)}
"
	case "$2" in pass) ;; *) failed=1 ;; esac
}

# check NAME COMMAND...: runs the command, keeps its output for the report.
check() {
	local name="$1"
	shift
	if "$@" >"$logs/$name.log" 2>&1; then
		record "$name" pass
	else
		record "$name" fail
		echo "--- $name: first lines of output" >&2
		head -n 20 "$logs/$name.log" >&2
	fi
}

# Prefer the copy `go install` writes (GOBIN, else the first GOPATH entry):
# it is built with this module's Go. An older copy on PATH fails to load the
# config.
find_lint() {
	local gobin gopath
	gobin="$(go env GOBIN)"
	gopath="$(go env GOPATH)"
	gopath="${gopath%%:*}"
	if [ -n "$gobin" ] && [ -x "$gobin/golangci-lint" ]; then
		echo "$gobin/golangci-lint"
	elif [ -n "$gopath" ] && [ -x "$gopath/bin/golangci-lint" ]; then
		echo "$gopath/bin/golangci-lint"
	elif command -v golangci-lint >/dev/null 2>&1; then
		command -v golangci-lint
	fi
}

gofmt_clean() { out="$(gofmt -l .)" && [ -z "$out" ] || { echo "$out"; return 1; }; }

lint_all() {
	"$lint" run ./... && GOOS=windows "$lint" run ./... && GOOS=darwin "$lint" run ./...
}

tooling() {
	python3 scripts/memory.py validate &&
		python3 -m unittest discover -s scripts/tests &&
		bash .claude/hooks/guard-git_test.sh &&
		bash .github/scripts/branch-name-check_test.sh &&
		bash .github/scripts/commit-identity-check_test.sh &&
		bash .github/scripts/release-check_test.sh &&
		bash scripts/verify_test.sh
}

e2e_run() {
	go build -o "$logs/truss" ./cmd/truss && bash .github/scripts/e2e-test.sh "$logs/truss"
}

install_hint="go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$lint_version"

if ! command -v go >/dev/null 2>&1; then
	for c in gofmt vet lint test e2e; do wanted "$c" && record "$c" missing "go not found"; done
else
	wanted gofmt && check gofmt gofmt_clean
	wanted vet && check vet go vet ./...
	if wanted lint; then
		lint="$(find_lint)"
		if [ -z "$lint" ]; then
			record lint missing "install: $install_hint"
		elif ! "$lint" version 2>/dev/null | grep -q "version $lint_version "; then
			record lint fail "$lint is not v$lint_version; install: $install_hint"
		else
			check lint lint_all
		fi
	fi
	wanted test && check test go test -race -count=1 ./...
fi

if wanted tooling; then
	if command -v python3 >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
		check tooling tooling
	else
		record tooling missing "needs python3 and jq"
	fi
fi

wanted e2e && command -v go >/dev/null 2>&1 && check e2e e2e_run

printf '%s' "$summary"
dirty=""
[ -n "$(git status --porcelain 2>/dev/null)" ] && dirty=" + uncommitted changes"
echo "commit    $(git rev-parse --short HEAD 2>/dev/null || echo none)$dirty"
if [ "$failed" -ne 0 ] || [ -z "$summary" ]; then
	echo "result    FAIL"
	exit 1
fi
echo "result    PASS"
