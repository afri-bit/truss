#!/usr/bin/env bash
# Usage: e2e-test.sh <path-to-truss-binary>
#
# Runs the built truss binary against the `truss run` contract in
# specs/001-host-pipeline-engine/contracts/cli.md and the run-log contract in
# specs/002-local-run-log/contracts/run-log.md (as extended by spec 003). Log
# lines start with a timestamp, so assertions match the text after it.
# The concurrency cases follow specs/004-step-concurrency/contracts/. Runs under bash on Linux,
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

# run_truss_limited DIR ARGS... is run_truss with a hard 60 second limit, so a
# regression that serialises steps that wait for each other fails instead of
# hanging. Uses timeout or gtimeout when present, else a background watchdog.
run_truss_limited() {
	local dir="$1"
	shift
	local tool=""
	if command -v timeout >/dev/null 2>&1; then
		tool=timeout
	elif command -v gtimeout >/dev/null 2>&1; then
		tool=gtimeout
	fi
	if [ -n "$tool" ]; then
		(cd "$dir" && "$tool" 60 "$truss" "$@" >"$work/.stdout" 2>"$work/.stderr")
		code=$?
	else
		(cd "$dir" && "$truss" "$@" >"$work/.stdout" 2>"$work/.stderr") &
		local pid=$!
		(sleep 60 && kill "$pid" 2>/dev/null) &
		local watchdog=$!
		wait "$pid"
		code=$?
		kill "$watchdog" 2>/dev/null
		wait "$watchdog" 2>/dev/null
	fi
	out="$(cat "$work/.stdout")"
	err="$(cat "$work/.stderr")"
}

# line_no PATTERN prints the 1-based line number in $out of the first
# timestamped line whose text matches the glob PATTERN, or nothing.
line_no() {
	local line n=0
	while IFS= read -r line; do
		n=$((n + 1))
		[[ "$line" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}\ (.*)$ ]] || continue
		# shellcheck disable=SC2053
		if [[ "${BASH_REMATCH[1]}" == $1 ]]; then
			echo "$n"
			return 0
		fi
	done <<<"$out"
	return 1
}

# mark_and_wait MARK OTHER prints a step command that creates the file MARK and
# then waits (bounded, about 20 seconds) until the file OTHER exists. It does
# not exit; the caller appends what happens next. OTHER may name a file that
# is never created.
mark_and_wait() {
	if $windows; then
		# The loop is parenthesized: in cmd, text after "if ... cmd &" on the same
		# line belongs to the if, so a chained command would be skipped.
		printf 'echo x> %s & (for /l %%i in (1,1,1000) do @if not exist %s ping -n 1 127.0.0.1 >nul)' "$1" "$2"
	else
		printf 'echo x > %s; i=0; while [ ! -f %s ] && [ $i -lt 400 ]; do i=$((i + 1)); sleep 0.05; done' "$1" "$2"
	fi
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
if [ "$code" -ne 1 ] || [[ "$err" != *"usage: truss <command> [arguments]"* ]]; then
	fail "usage error" "want exit 1 and the usage line"
else
	pass "usage error"
fi
rm -rf "$work"

# Steps that wait for each other run together under pipeline.concurrency.
new_dir
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: together
  concurrency: 2

steps:
  - id: a
    run: '$(mark_and_wait a.ready b.ready)'
  - id: b
    run: '$(mark_and_wait b.ready a.ready)'
EOF
run_truss_limited "$work/project" run
a_started="$(line_no "step a started")"
b_started="$(line_no "step b started")"
a_finished="$(line_no "step a finished duration *s exit 0")"
b_finished="$(line_no "step b finished duration *s exit 0")"
if [ "$code" -ne 0 ]; then
	fail "concurrent steps" "want exit 0"
elif [ -z "$a_started" ] || [ -z "$b_started" ] || [ -z "$a_finished" ] || [ -z "$b_finished" ]; then
	fail "concurrent steps" "missing started or finished line"
elif [ "$a_started" -gt "$a_finished" ] || [ "$a_started" -gt "$b_finished" ] ||
	[ "$b_started" -gt "$a_finished" ] || [ "$b_started" -gt "$b_finished" ]; then
	fail "concurrent steps" "a step finished before both had started"
elif ! has_line "summary succeeded steps 2 duration *s"; then
	fail "concurrent steps" "missing succeeded summary with steps 2"
else
	pass "concurrent steps"
fi
rm -rf "$work"

# A failed step stops the running steps and starts no more.
new_dir
# Commands are chained with ; for sh and & for cmd.
chain=";"
bad_exit="exit 3"
slow_then="echo x > slow-ran.txt"
never_cmd="echo x > never-ran.txt"
if $windows; then
	chain="&"
	bad_exit="exit 3"
	slow_then="echo x> slow-ran.txt"
	never_cmd="echo x> never-ran.txt"
fi
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: stop-rest
  concurrency: 2

steps:
  - id: slow
    run: '$(mark_and_wait slow.started never.flag) $chain $slow_then'
  - id: bad
    run: '$(mark_and_wait bad.started slow.started) $chain $bad_exit'
  - id: never
    needs: [bad]
    run: '$never_cmd'
EOF
run_truss_limited "$work/project" run
if [ "$code" -ne 1 ]; then
	fail "stop on failure" "want exit 1"
elif ! has_line "error stop-rest failed at bad (exit 3)"; then
	fail "stop on failure" "missing failed step error line"
elif ! has_line "step slow stopped because step bad failed"; then
	fail "stop on failure" "missing stopped line for slow"
elif has_line "step slow finished duration *s exit 0"; then
	fail "stop on failure" "slow finished successfully"
elif has_line "step never started"; then
	fail "stop on failure" "never started"
elif [ -e "$work/project/slow-ran.txt" ] || [ -e "$work/project/never-ran.txt" ]; then
	fail "stop on failure" "a stopped or later step ran to its marker"
else
	pass "stop on failure"
fi
rm -rf "$work"

# A misspelled concurrency field is rejected and leaves the run log alone.
new_dir
mkdir "$work/project/.truss"
printf '2000-01-01 00:00:00 earlier run\n' >"$work/project/.truss/run.log"
cp "$work/project/.truss/run.log" "$work/before.log"
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: typo
  concurency: 2

steps:
  - id: touch
    run: echo ran> marker.txt
EOF
run_truss "$work/project" run
if [ "$code" -ne 1 ]; then
	fail "unknown field" "want exit 1"
elif [ "$err" != 'error: unknown field "concurency" in pipeline' ]; then
	fail "unknown field" "want the unknown field error on stderr"
elif [ -e "$work/project/marker.txt" ]; then
	fail "unknown field" "a step command ran"
elif ! cmp -s "$work/before.log" "$work/project/.truss/run.log"; then
	fail "unknown field" "run log changed"
else
	pass "unknown field"
fi
rm -rf "$work"

# pipeline.workdir: steps start in that folder, and the data folders exist.
new_dir
mkdir "$work/project/app"
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: workdir
  workdir: app

steps:
  - id: touch
    run: echo ran> marker.txt
EOF
run_truss "$work/project" run
if [ "$code" -ne 0 ]; then
	fail "workdir" "want exit 0"
elif [ ! -e "$work/project/app/marker.txt" ] || [ -e "$work/project/marker.txt" ]; then
	fail "workdir" "the step did not run in app"
elif [ ! -d "$work/project/.truss/data/cache" ]; then
	fail "workdir" "the cache folder was not created"
else
	pass "workdir"
fi
sed -i.bak 's/workdir: app/workdir: missing/' "$work/project/truss.yaml"
run_truss "$work/project" run
if [ "$code" -ne 1 ] || ! has_line "summary failed *"; then
	fail "workdir missing" "want exit 1 and a failed summary"
else
	pass "workdir missing"
fi
rm -rf "$work"

# Jobs: the steps of a job run in order and a job waits for the jobs it needs.
new_dir
cat >"$work/project/truss.yaml" <<EOF
version: "1"

pipeline:
  name: jobs
  concurrency: 2

env:
  STAGE: e2e

jobs:
  build:
    steps:
      - id: compile
        run: echo compiling
      - id: package
        run: echo packaging
  ship:
    needs: [build]
    steps:
      - id: publish
        run: echo publishing
EOF
run_truss "$work/project" run
if [ "$code" -ne 0 ]; then
	fail "jobs" "want exit 0"
elif ! has_line "step build/package | packaging" || ! has_line "step ship/publish | publishing"; then
	fail "jobs" "missing job step output"
elif ! has_line "summary succeeded steps 3 duration *"; then
	fail "jobs" "missing summary"
else
	pass "jobs"
fi
run_truss "$work/project" validate
if [ "$code" -ne 0 ] || [[ "$out" != *"step ship/publish needs build/package"* ]]; then
	fail "validate" "want exit 0 and the flattened steps"
else
	pass "validate"
fi

# The daemon: start it, register the pipeline, run it, read the history, stop it.
export TRUSS_HOME="$work/home"
run_truss "$work/project" daemon start --address 127.0.0.1:0
if [ "$code" -ne 0 ] || [[ "$out" != "daemon started"* ]]; then
	fail "daemon start" "want exit 0 and the started line"
else
	pass "daemon start"
	run_truss "$work/project" register
	if [ "$code" -ne 0 ] || [[ "$out" != "registered jobs "* ]]; then
		fail "register" "want exit 0 and the registered line"
	else
		pass "register"
	fi
	run_truss "$work/project" trigger jobs --follow
	if [ "$code" -ne 0 ] || ! has_line "step ship/publish | publishing" || ! has_line "summary succeeded steps 3 duration *"; then
		fail "trigger" "want exit 0 and the run log"
	else
		pass "trigger"
	fi
	run_truss "$work/project" runs jobs
	if [ "$code" -ne 0 ] || [[ "$out" != *"succeeded"*"manual"* ]]; then
		fail "runs" "want the finished run in the history"
	else
		pass "runs"
	fi
	run_truss "$work/project" unregister jobs
	run_truss "$work/project" pipelines
	if [ "$code" -ne 0 ] || [[ "$out" != "no pipelines registered"* ]]; then
		fail "unregister" "want an empty registry"
	else
		pass "unregister"
	fi
	run_truss "$work/project" daemon stop
	if [ "$code" -ne 0 ] || [ "$out" != "daemon stopped" ]; then
		fail "daemon stop" "want exit 0 and the stopped line"
	else
		pass "daemon stop"
	fi
fi
run_truss "$work/project" daemon status
if [ "$code" -ne 1 ]; then
	fail "daemon status" "want exit 1 when no daemon runs"
else
	pass "daemon status"
fi
unset TRUSS_HOME
rm -rf "$work"

if [ "$failures" -gt 0 ]; then
	echo "$failures end-to-end test(s) failed"
	exit 1
fi
echo "all end-to-end tests passed"
