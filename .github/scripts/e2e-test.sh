#!/usr/bin/env bash
# Usage: e2e-test.sh <path-to-truss-binary>
#
# Runs the built truss binary against the `truss run` contract in
# specs/001-host-pipeline-engine/contracts/cli.md and the run-log contract in
# specs/002-local-run-log/contracts/run-log.md (as extended by spec 003). Log
# lines start with a timestamp, so assertions match the text after it.
# Runs under bash on Linux,
# macOS, and Git Bash on Windows, where steps execute through cmd /c.
set -u

if [ $# -ne 1 ]; then
	echo "usage: e2e-test.sh <path-to-truss-binary>" >&2
	exit 2
fi
truss="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"

windows=false
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN*) windows=true ;;
esac

failures=0
work=""
out=""
err=""
code=0

pass() { echo "ok   $1"; }
fail() {
	echo "FAIL $1: $2"
	echo "  exit: $code"
	echo "  stdout: $out"
	echo "  stderr: $err"
	failures=$((failures + 1))
}

# run_truss DIR ARGS... runs truss in DIR and captures exit, stdout, stderr.
run_truss() {
	local dir="$1"
	shift
	(cd "$dir" && "$truss" "$@" >"$work/.stdout" 2>"$work/.stderr")
	code=$?
	out="$(cat "$work/.stdout")"
	err="$(cat "$work/.stderr")"
}

# has_line PATTERN succeeds when a line of $out is a timestamp
# (YYYY-MM-DD HH:MM:SS and a space) followed by text matching the shell glob
# PATTERN for the whole rest of the line.
has_line() {
	local line
	while IFS= read -r line; do
		[[ "$line" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}\ (.*)$ ]] || continue
		# shellcheck disable=SC2053
		[[ "${BASH_REMATCH[1]}" == $1 ]] && return 0
	done <<<"$out"
	return 1
}

new_dir() {
	work="$(mktemp -d)"
	mkdir "$work/project"
}

hello_pipeline() {
	cat <<'EOF'
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo Hello from TRUSS
EOF
}

second_fails="exit 1"
if $windows; then
	second_fails="exit /b 1"
fi

# Explicit path with any filename succeeds and streams the step log.
new_dir
hello_pipeline >"$work/project/custom.yaml"
run_truss "$work/project" run custom.yaml
if [ "$code" -ne 0 ]; then
	fail "explicit path" "want exit 0"
elif ! has_line "step hello started" || ! has_line "step hello | Hello from TRUSS" ||
	! has_line "step hello finished duration *s exit 0"; then
	fail "explicit path" "missing step log"
elif ! has_line "summary succeeded steps 1 duration *s"; then
	fail "explicit path" "missing succeeded summary"
elif [[ "$out" == *"status:"* || "$out" == *"exit_code:"* ]]; then
	fail "explicit path" "success transcript has a status block"
else
	pass "explicit path"
fi
rm -rf "$work"

# Bare `truss run` loads truss.yaml from the working directory.
new_dir
hello_pipeline >"$work/project/truss.yaml"
run_truss "$work/project" run
if [ "$code" -ne 0 ] || ! has_line "step hello | Hello from TRUSS"; then
	fail "default truss.yaml" "want exit 0 and the hello step"
else
	pass "default truss.yaml"
fi
rm -rf "$work"

# Bare `truss run` does not fall back to pipeline.yaml.
new_dir
hello_pipeline >"$work/project/pipeline.yaml"
run_truss "$work/project" run
if [ "$code" -ne 1 ] || [[ "$err" != "error:"* ]] || has_line "step hello started"; then
	fail "missing truss.yaml" "want exit 1, an error line, and no step"
else
	pass "missing truss.yaml"
fi
rm -rf "$work"

# An invalid definition is rejected before any step runs.
new_dir
cat >"$work/project/bad.yaml" <<EOF
pipeline:
  name: no-version
steps:
  - id: touch
    run: echo ran> marker.txt
EOF
run_truss "$work/project" run bad.yaml
if [ "$code" -ne 1 ] || [[ "$err" != "error:"* ]] || has_line "step touch started"; then
	fail "invalid definition" "want exit 1, an error line, and no step"
elif [ -e "$work/project/marker.txt" ]; then
	fail "invalid definition" "a step command ran"
else
	pass "invalid definition"
fi
rm -rf "$work"

# Invalid YAML is rejected.
new_dir
printf 'version: "1"\nsteps: [\n' >"$work/project/broken.yaml"
run_truss "$work/project" run broken.yaml
if [ "$code" -ne 1 ] || [[ "$err" != "error:"* ]]; then
	fail "invalid YAML" "want exit 1 and an error line"
else
	pass "invalid YAML"
fi
rm -rf "$work"

# A failed step stops the pipeline, keeps its exit code, and skips later steps.
new_dir
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: fail-fast

steps:
  - id: first
    run: echo first
  - id: second
    run: $second_fails
  - id: third
    run: echo third> marker.txt
EOF
run_truss "$work/project" run
if [ "$code" -ne 1 ]; then
	fail "fail-fast" "want exit 1"
elif ! has_line "step first finished duration *s exit 0" ||
	! has_line "step second finished duration *s exit 1" || has_line "step third started"; then
	fail "fail-fast" "want first and second reported, third absent"
elif ! has_line "summary failed steps 2 duration *s"; then
	fail "fail-fast" "missing failed summary"
elif ! has_line "error fail-fast failed at second (exit 1)"; then
	fail "fail-fast" "missing failed step error line"
elif [ -e "$work/project/marker.txt" ]; then
	fail "fail-fast" "third step ran"
else
	pass "fail-fast"
fi
rm -rf "$work"

# Any other invocation is a usage error that executes nothing.
new_dir
run_truss "$work/project" build
if [ "$code" -ne 1 ] || [[ "$err" != *"usage: truss run [<path>]"* ]]; then
	fail "usage error" "want exit 1 and the usage line"
else
	pass "usage error"
fi
rm -rf "$work"

if [ "$failures" -gt 0 ]; then
	echo "$failures end-to-end test(s) failed"
	exit 1
fi
echo "all end-to-end tests passed"
