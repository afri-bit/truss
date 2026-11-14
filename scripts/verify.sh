#!/usr/bin/env bash
# Usage: scripts/verify.sh [--e2e] [--only name,name,...]
#
# TRUSS's local quality gate, the same checks CI runs. Prints one line per
# check and exits 1 if any check fails or its tool is missing, 0 otherwise.
# A missing tool is reported as "missing" and never counts as a pass.
#
# Checks: gofmt, vet, lint (Linux, Windows and macOS files), test (-race),
# tooling (memory validation, Python and shell tests), e2e (built binary).
# e2e runs with --e2e, or when the branch changes cmd/, internal/executor/,
# internal/runlog/ or .github/scripts/ compared with develop.
#
# The last line names the commit the result belongs to; paste the output into
# the pull request as verification evidence. Must stay compatible with bash 3.2.
set -u

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root" || exit 2

e2e=auto
only=""
while [ $# -gt 0 ]; do
	case "$1" in
	--e2e) e2e=yes ;;
	--only)
		[ $# -ge 2 ] || { echo "usage: verify.sh [--e2e] [--only name,...]" >&2; exit 2; }
		only=",$2,"; shift ;;
	-h | --help) sed -n '2,15p' "$0"; exit 0 ;;
	*) echo "usage: verify.sh [--e2e] [--only name,...]" >&2; exit 2 ;;
	esac
	shift
done

logs="$(mktemp -d)"
trap 'rm -rf "$logs"' EXIT
failed=0
summary=""

wanted() { [ -z "$only" ] || case "$only" in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

# record NAME RESULT [DETAIL]
record() {
	summary="$summary$(printf '%-9s %s' "$1" "$2")${3:+  ($3)}
"
	case "$2" in pass | skipped) ;; *) failed=1 ;; esac
}

# check NAME COMMAND...: runs the command, keeps its output for the report.
check() {
	local name="$1"; shift
	if "$@" >"$logs/$name.log" 2>&1; then
		record "$name" pass
	else
		record "$name" fail
		echo "--- $name: first lines of output" >&2
		head -n 20 "$logs/$name.log" >&2
	fi
}

# Prefer the copy `go install` put in GOPATH/bin: it is built with this
# module's Go. An older copy elsewhere on PATH fails to load the config.
find_lint() {
	if [ -x "$(go env GOPATH)/bin/golangci-lint" ]; then
		echo "$(go env GOPATH)/bin/golangci-lint"
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

if ! command -v go >/dev/null 2>&1; then
	for c in gofmt vet lint test e2e; do wanted "$c" && record "$c" missing "go not found"; done
else
	wanted gofmt && check gofmt gofmt_clean
	wanted vet && check vet go vet ./...
	if wanted lint; then
		lint="$(find_lint)"
		if [ -z "$lint" ]; then
			record lint missing "install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
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

if wanted e2e && command -v go >/dev/null 2>&1; then
	if [ "$e2e" = auto ]; then
		base="$(git merge-base HEAD origin/develop 2>/dev/null || git merge-base HEAD develop 2>/dev/null || true)"
		if [ -n "$base" ] && git diff --name-only "$base" HEAD |
			grep -qE '^(cmd/|internal/executor/|internal/runlog/|\.github/scripts/)'; then
			e2e=yes
		fi
	fi
	if [ "$e2e" = yes ]; then check e2e e2e_run; else record e2e skipped "CLI paths unchanged; --e2e to force"; fi
fi

printf '%s' "$summary"
dirty=""
[ -n "$(git status --porcelain 2>/dev/null)" ] && dirty=" + uncommitted changes"
echo "commit    $(git rev-parse --short HEAD 2>/dev/null || echo unknown)$dirty"
if [ "$failed" -ne 0 ]; then
	echo "result    FAIL"
	exit 1
fi
echo "result    PASS"
