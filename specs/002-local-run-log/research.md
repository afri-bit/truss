# Research: Local Run Log

## Transcript owner

- **Decision**: `internal/runlog` writes the run-log bytes. `cmd/truss` passes that writer to standard output and to a temporary file. The engine emits step events and does not format lines.
- **Rationale**: The spec requires the saved file to be the same text the developer read. One writer produces those bytes, so the terminal and the file cannot drift. The constitution keeps execution independent of presentation, so the engine does not import `runlog`.
- **Alternatives considered**: Format the transcript inside the engine (rejected: the engine would own CLI text). Format only after the run returns (rejected: step output must appear as it is produced, with a timestamp on each line).

## Step events

- **Decision**: The engine accepts an interface with `StepStarted(id)`, `StepOutput(id, chunk)`, and `StepFinished(id, exitCode, startErr)`. A nil interface skips those calls. The CLI calls setup before `Run` and the summary after `Run` returns a pipeline result.
- **Rationale**: The CLI already knows the operating system, architecture, shell, and working directory. The engine knows when a step starts, when output arrives, and when it finishes. Splitting those calls keeps the file path and the help commands out of the engine.
- **Alternatives considered**: Pass only an `io.Writer` and let the engine print lines (rejected: that is the current transcript, which this feature replaces).

## Timestamp and duration

- **Decision**: Each line starts with the local time `2006-01-02 15:04:05`. The writer remembers the previous timestamp and repeats it when the clock moves backward. Duration is finish time minus start time, rounded to the nearest millisecond, printed as seconds with three digits (`0.040s`, `65.200s`). A step's duration is that step's interval. The run's duration is the summary time minus the setup time.
- **Rationale**: The spec fixes the timestamp form and requires duration beside it. One duration form covers steps under and over a minute. Repeating the previous timestamp satisfies the rule that a timestamp does not move backward, including two occurrences in the same second.
- **Alternatives considered**: RFC3339 with a numeric offset (rejected: the spec fixes `YYYY-MM-DD HH:MM:SS`). Minute-and-second duration after 60 seconds (rejected: the spec says seconds). Omitting a fractional part after one minute (rejected: one format is easier to test and still shows the fractional seconds the spec requires under a minute).

## Clock

- **Decision**: The run log takes a clock function. The CLI passes `time.Now`. Tests pass a clock they control.
- **Rationale**: Contract tests need ordered timestamps without sleeping, and they need a backward step to prove the timestamp does not move earlier.
- **Alternatives considered**: Read the wall clock inside the formatter only (rejected: tests would assert a pattern instead of the order the spec requires).

## Operating system, architecture, shell, and directory

- **Decision**: The setup lines use `linux`, `macOS`, or `windows` for those three hosts, and the Go OS name for any other host. Architecture is the Go architecture name (`amd64`, `arm64`, and so on). The shell is the path the host executor uses (`/bin/sh`, or `COMSPEC` when set, otherwise `cmd.exe`). The directory is the absolute invoker working directory, with that host's separators. `executor` exposes the shell path so the log and the executor cannot disagree.
- **Rationale**: The spec asks for the operating system, not the kernel name `darwin`. The shell and directory must be the ones the steps actually use. Absolute path makes the directory recognizable when the pipeline file lives elsewhere.
- **Alternatives considered**: Print `runtime.GOOS` unchanged (rejected: `darwin` is not the operating system name). Print the shell command line including the step text (rejected: that repeats the command and is not the setup fact).

## Latest run log file

- **Decision**: After validation succeeds, and before any step, the CLI creates `.truss` in the working directory when it is missing and opens `.truss/run.log.tmp`. It does not open or truncate `.truss/run.log` yet. The writer copies every run-log byte to standard output and to that temporary file. When the run finishes, successfully or as a failed pipeline, the CLI closes the temporary file and renames it to `.truss/run.log`. A rejected definition never creates or replaces those files. If `.truss` cannot be created, or the temporary file cannot be opened, the CLI prints an error and starts no step. If the final rename fails, the CLI prints an error, exits non-zero, and leaves the previous `.truss/run.log` in place.
- **Rationale**: Validation failures must leave the previous log unchanged. A same-directory rename is what makes the finished log appear as one file. Checking the temporary file before steps matches the requirement to discover an unwritable location before execution.
- **Alternatives considered**: Append to `.truss/run.log` during the run (rejected: a failed save or a crash would destroy or mix the previous log). Keep a history directory (rejected: the spec keeps one latest log). Write the file from memory only at the end (rejected: the live terminal and the file must share one writer so they stay identical). Edit `.gitignore` (rejected: the spec forbids it).

## Help, version, and errors

- **Decision**: `truss`, `truss help`, and `truss --help` print usage to standard output and exit 0. `truss version` and `truss --version` print one line, the `// version:` value from `go.mod`, to standard output and exit 0. The CLI stores that value in a string constant. A test fails when the constant and the `go.mod` line differ. Any other invocation prints usage to standard error and exits 1. `truss run --help` remains a path. Load, parse, and validation errors stay on standard error as `error: <diagnostic>` and write no run log. The failed-step line is part of the run log, not a second copy on standard error.
- **Rationale**: Help and version are successful questions, so they use standard output and exit 0. The version cannot be read from `go.mod` at runtime after installation. The spec puts the failed-step name in the run log that is saved, so that line belongs to the same writer.
- **Alternatives considered**: Parse `go.mod` at runtime (rejected: installed binaries do not ship the module file). Keep the failure line only on standard error (rejected: the saved log would omit it).

## What stays

- **Decision**: Schema version `"1"`, unknown-field handling, sequential fail-fast, host-shell `run` strings, and the process exit for a succeeded or failed pipeline stay as specified in `specs/001-host-pipeline-engine`. No secret store and no environment dump are added.
- **Rationale**: This feature changes the unreleased transcript and the no-command exit. It does not change execution semantics or the pipeline file.
- **Alternatives considered**: None. The spec lists those as unchanged.
