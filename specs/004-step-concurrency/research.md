# Research: Step Concurrency

## Where the limit and the strictness live

- **Decision**: Add `Concurrency int` to `pipeline.Pipeline` (0 means "not declared", which means 1) and copy a normalized value into `ExecutionPlan`. Keep the existing `yaml.v3` node walk and add a `default:` branch at the top level, under `pipeline`, and in each Step that records the first unknown key with its location. Validate reports it.
- **Rationale**: `KnownFields` only applies when decoding into structs; the parser deliberately walks nodes so it can distinguish missing, null, and wrongly typed values for `version`, `run`, and `needs`. Engine tests build `Pipeline` literals, so the zero value must stay valid.
- **Alternatives considered**: Decode into structs with `KnownFields(true)` (rejected: loses the explicit shape checks and their messages). A pointer field for "declared" (rejected: the zero value already means "use 1", and a declared `0` is rejected at parse, not stored).

## What counts as a valid limit

- **Decision**: Accept only a scalar with tag `!!int` whose text is decimal digits (`^[0-9]+$`) and at least 1. Reject null, empty, `!!str` (including `"3"`), `!!bool`, `!!float` (`1.5`, `1.0`), sequences, mappings, and `0x10`, `0o7`, `1_000`, `+3`. A value too large for `int` is clamped to the maximum, which means "as many as are ready".
- **Rationale**: The schema must be readable by other languages (constitution V); decimal-only avoids YAML 1.1/1.2 differences in octal, hex, and underscores. The spec says there is no upper bound.
- **Alternatives considered**: Accept quoted numbers (rejected: FR-003 lists text as invalid). Accept `auto` (rejected: out of scope).

## Validation order

- **Decision**: `version`, then unknown field, then `name`, then `concurrency`, then steps, `id`, `run`, then `needs`. Existing messages do not change. New messages: `unknown field "x" at top level`, `unknown field "x" in pipeline`, `unknown field "x" in step "id"` (or `in step N`, 1-based, when the id is missing), and `pipeline concurrency must be a whole number of at least 1, got "X"` (`got a list`, `got a mapping` for non-scalars).
- **Rationale**: The version decides which fields are known. A valid limit with a bad graph must still yield the existing Dependency errors (Story 4, scenario 3).
- **Alternatives considered**: Collect all problems (rejected: the validator returns one error today).

## Scheduling

- **Decision**: One scheduler goroutine owns all state. Loop: while no failure is recorded and the context is live, scan `p.Steps` in definition order and start every Step that has not started, whose needs have all succeeded, until `running == limit`. Start synchronously (so the log's start order is the scan order), reserve the Step's slot in `run.Steps`, and run `Wait` in a goroutine that sends its outcome on a channel. Block on that channel; on each outcome emit events, fill the slot, and on the first failure record it and cancel the run context. At limit 1 this equals spec 003: with fail-fast, "succeeded" equals "placed" in `Plan`, and both pick the earliest eligible Step.
- **Rationale**: Recording failures and gating starts in the same goroutine removes the race where a Step starts after the stop signal. One code path for every limit cannot drift from the sequential contract. `Plan` is still called and still validates reachability.
- **Alternatives considered**: Keep the old loop for limit 1 and add a scheduler for >1 (rejected: two paths to keep in agreement). A worker pool with a shared queue (rejected: harder to prove definition-order starts and no-start-after-failure).

## "First observed failure"

- **Decision**: The first failure recorded by the scheduler goroutine: a synchronous `Start` error, or the first failed outcome received. `PipelineRun.FailedStep` carries its id and the CLI error line uses it; the log keeps every Step's own Exit Code or diagnostic.
- **Rationale**: Wall-clock ordering across goroutines is not observable; the scheduler's order is deterministic for a given interleaving and is what FR-008 asks for. Today's "last entry in `run.Steps`" is wrong under concurrency.
- **Alternatives considered**: Earliest finish timestamp (rejected: timestamps can tie and are formatted to the second).

## Stopping a Step: Linux and macOS

- **Decision**: Start the shell with `SysProcAttr{Setpgid: true}` so it leads a new process group (pgid = pid). To stop, `syscall.Kill(-pgid, SIGKILL)`, ignoring `ESRCH`. After the shell exits (for any reason), kill the group again to end leftovers, then poll `Kill(-pgid, 0)` until `ESRCH`, bounded to about one second, so descendants are gone before `Wait` returns. Use `exec.Command`, not `CommandContext`; `Wait` selects between the shell's exit and the context.
- **Rationale**: The group is created at `fork` and so contains every descendant that does not change its own group. Killing the group is the only standard-library way to end the whole tree. Ending leftovers when the shell exits (maintainer decision) keeps the window small: the group id is reusable only if the shell and every descendant are already gone, in which case the later `kill` is aimed at an id that nothing in that group owns. A recycled id that a new group leader picked up in that few-microsecond window could be hit. Go's standard library cannot wait for the shell without reaping it (`waitid` with `WNOWAIT` needs `golang.org/x/sys`), so this residual risk is accepted and recorded.
- **Alternatives considered**: `Process.Kill` on the shell only (rejected: leaves children, violates FR-014). Parsing `/proc` (rejected: not portable to macOS). cgroups or a subreaper (rejected: privileged or platform-specific; it would close the `setsid` escape but at too high a cost).

## Stopping a Step: Windows

- **Decision**: Start the shell with `CREATE_SUSPENDED`. Create a job object (`CreateJobObjectW`), assign the suspended process (`AssignProcessToJobObject`), then resume it (`NtResumeProcess` from `ntdll`). Descendants are created inside the job and cannot break away because `JOB_OBJECT_LIMIT_BREAKAWAY_OK` is not set. To stop or to clean up, `TerminateJobObject(job, 21586)` (a distinctive exit code, so `Stopped` can tell a kill from a Step that exited by itself), then close the handle. Unlike Unix, TRUSS does not poll for the job to be empty; termination is effectively synchronous and the tests poll with a bound. All calls go through `syscall.NewLazyDLL` / `LazyProc.Call`. If any job call fails, kill the suspended process and report a start failure; user code never ran.
- **Rationale**: A job object is the Windows mechanism for owning a process tree. Suspended start closes the window in which a child could be created before the shell is in the job. `kernel32` and `ntdll` are KnownDLLs, so loading them by name cannot be hijacked. Nested jobs work on Windows 8 and later, which covers hosted CI runners.
- **Alternatives considered**: `golang.org/x/sys/windows` (rejected: a new third-party module, which needs a maintainer decision, and it does not remove the resume problem). `taskkill /T` (rejected: spawns a command TRUSS did not declare, violating "no implicit command execution"). Toolhelp32 + `ResumeThread` instead of `NtResumeProcess` (documented alternative if the undocumented call is a concern; more code).

## Exit Code of a stopped or signalled Step

- **Decision**: On Unix, a process ended by a signal has no Exit Code; the finish line shows the diagnostic `terminated by signal: killed` (from the signal's name). On Windows a job-terminated process reports the distinctive exit code 21586 (0x5452), which the log shows as `exit 21586`; `Stopped` is true only when the shell's exit code is that value. `Result.Stopped` marks that TRUSS ended the tree before the shell exited and the exit was not a success; a Step that exits 0 on its own while a stop races is reported as succeeded without a `stopped` line.
- **Rationale**: A made-up `-1` was Go's sentinel, not a host code (spec FR-011). Windows has no signal concept. Exit code 1 (what `Process.Kill` uses) cannot be told from a Step that fails with 1 by itself, which CI showed as a wrongly stopped Step, so a distinctive code is used.
- **Alternatives considered**: `128+n` shell convention on Unix (rejected: invents a code the host did not report).

## Run log thread safety

- **Decision**: Add a `sync.Mutex` to `runlog.Log`; every exported method locks it for the whole format, stamp, and write. The existing per-Step `pending` buffers already hold partial lines. `StepStopped(id, by)` flushes the Step's partial line, then writes the stopped line; `StepFinished` follows. The CLI's single `io.MultiWriter(stdout, file)` is only written under that lock, so the shown and saved text are identical.
- **Rationale**: One critical section per line gives whole lines, per-Step order, and monotonic timestamps with no other change.
- **Alternatives considered**: A channel-based writer goroutine (rejected: more machinery, same guarantee).

## Interrupt

- **Decision**: `main` uses `signal.NotifyContext(os.Interrupt, syscall.SIGTERM)`. The engine already returns `ctx.Err()` and the CLI prints `error: <err>`, exits 1, and discards the temporary log. Running Steps are ended through the same context path as a failure.
- **Rationale**: With `Setpgid`, the terminal's Ctrl-C no longer reaches Steps, so without this an interrupted `truss` would orphan them.
- **Alternatives considered**: Leave interrupt undefined (rejected by the maintainer).

## Testing technique

- **Decision**: A helper process: `TestMain` re-executes the test binary when `TRUSS_TEST_HELPER` is set, with subcommands `touch-and-wait <mine> <other>`, `print-lines`, `partial`, `spawn-grandchild <pidfile>`, and `exit <code>`. Rendezvous is file existence with a deadline that only turns a wrong implementation into a failure. Prove "nothing outlives `truss`" by having the grandchild write its pid and block, then asserting the pid is gone after `Run` returns (Unix `kill(pid, 0)`; Windows `OpenProcess` plus `GetExitCodeProcess`). Concurrency, stop, and tree tests run with `-race -count=20` in CI.
- **Rationale**: A plain `run` string that invokes the test binary works the same under `/bin/sh -c` and `cmd /c`; no sleeps as synchronization.
- **Alternatives considered**: `sleep`, `touch`, `kill` shell commands (rejected: differ between `sh` and `cmd.exe`).
