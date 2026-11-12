# Changelog

All notable changes to TRUSS are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Dates below are when the work landed on `main`. No version has been tagged yet, so there is no released section.

## [Unreleased]

### Changed

- Queued runs survive a daemon restart. They stay `queued` when the daemon stops and start in request order when it starts again, instead of being cancelled. A queued run whose pipeline can no longer be loaded is closed as `failed`. Runs that were running are cancelled on a graceful stop, or closed as `failed` after a crash, as before.
- `truss help` and usage errors print the full command list. The first usage line is now `usage: truss <command> [arguments]`.
- Project process: constitution 2.0.0. A specification travels with the code instead of gating it, and tests ship with the behavior (see `docs/decisions/0005-specs-record-they-do-not-gate.md`).
- Require Go 1.27.3. Go 1.22.5's standard library is affected by GO-2025-3956 (`os/exec` LookPath) and GO-2025-3750 (`os` and `syscall`), which the vulnerability scan reports through the host executor and CLI.
- Processes a Step starts are ended when that Step's shell exits, so a Step can no longer leave a background process running for a later Step.
- A Step ended by a signal is logged with a diagnostic and no Exit Code instead of `exit -1`.
- On Linux and macOS each Step runs in its own process group so TRUSS can end it with everything it started. A Step that waits for terminal input (a password prompt) is stopped by the terminal until `truss` is interrupted. Interrupting `truss` (Ctrl-C or SIGTERM) now ends the running Steps and exits with status 1.
- Unknown fields in a pipeline definition are now rejected, at the top level, under `pipeline`, and in a Step. Before, they were ignored. The error names the field and where it appears.

### Added

- **Tracked local runs.** `truss run --track` still executes in your shell with your environment, and also reports the run to the daemon: it shows up in `truss runs` and TRUSS Desktop with the trigger `local`, its steps and its log. It needs a running daemon and fails with the usual hint before running anything if there is none; if the daemon goes away mid-run, the run finishes normally with one warning. Cancelling a tracked run in the daemon or Desktop stops the local run. Desktop shows a `local` badge. New API v1 endpoints, additive: `POST /runs`, `POST /runs/{id}/steps`, `POST /runs/{id}/log?offset=`, `POST /runs/{id}/finish` (see `docs/decisions/0006-tracked-local-runs.md`).
- **The daemon.** `truss daemon start`, `stop`, `status` and `run` manage a background service that listens on `127.0.0.1:7878`. It keeps its state under `~/.truss` (or `TRUSS_HOME`) and protects its HTTP API (`/api/v1`) with a token only your user can read.
- **Registered pipelines.** `truss register [<path>] [--name <name>]`, `unregister`, `pipelines`, `pause` and `resume`. The daemon stores the file's path, reads it again before every run, and notices edits.
- **Daemon runs.** `truss trigger <name> [--follow]`, `runs`, `logs <run-id> [--follow]` and `cancel`. Runs queue up; `--max-runs` limits how many execute at once (default 4). The newest 200 runs and their logs are kept.
- **Schedules.** `on.schedule` lists cron triggers with an optional `timezone` and a `missed` policy. A time that passed while the machine was off or asleep is skipped and recorded by default; `missed: run` starts one catch-up run.
- **Jobs.** `jobs` groups ordered steps under a name. Jobs wait for the jobs in their `needs`. In logs a job's step is named `job/step`.
- **Step settings.** `env` at pipeline, job and step level; `dir` and `timeout` per step. A step that exceeds its timeout fails with `timed out after <duration>`.
- **Overlap policy.** `pipeline.overlap` is `queue` (default), `skip` or `parallel`.
- **`truss validate [<path>]`** checks a definition and prints its schedules and steps without running anything.
- **TRUSS Desktop.** `truss desktop` opens a dashboard served by the daemon: counters, active and upcoming runs, pipeline controls, and a run page with the step graph and a live log.
- Runs started by the daemon set `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and `TRUSS_TRIGGER`.
- A cancelled daemon run closes its log with `summary cancelled steps <n> duration <d>`.
- Documentation: pipeline file reference, command line reference, daemon guide, TRUSS Desktop guide and an architecture page under `docs/`.
- Local run log in `.truss/run.log`. Each run shows a setup section, each Step's start, output, finish, duration and exit code, and a summary. The saved text is the text shown during the run. The latest log is replaced by the next finished run (specs/002-local-run-log).
- `truss help` and `truss version` (and `--help` and `--version`). They print usage or the version, exit 0, and start no Step.
- Optional `needs` on a Step. A Step starts only after every Step it lists has succeeded. Dependencies are validated before any command runs: unknown ids, self-dependencies, repeated ids, cycles, and a `needs` value that is not a list of ids are rejected (specs/003-step-dependencies).
- Optional `pipeline.concurrency`, a whole number of at least 1 that defaults to 1. Ready Steps start in definition order, never more than the limit at once. Any other value is rejected before a command starts (specs/004-step-concurrency).
- A `stopped` run-log line, `step <id> stopped because step <other> failed`, for a running Step that is stopped after another Step fails.
- Interrupt handling. Ctrl-C or SIGTERM ends every running Step, `truss` exits 1, and no log is saved for the interrupted run.
- Process-tree cleanup. A stopped Step's whole process tree is ended on Linux, macOS, and Windows. Processes a Step starts are ended when its shell exits.
- Go module `github.com/afri-bit/truss` for Go 1.27.3, and the `truss` command (`cmd/truss`).
- Pipeline schema version `"1"`. A definition has `version`, `pipeline.name`, and one or more `steps`. Each step has a unique `id` and a single `run` string. The host shell interprets that string, including quotes, pipes, and environment expansion.
- `truss run <path>` loads the given file. Any filename is valid. `truss run` with no path loads `truss.yaml` from the working directory.
- Sequential host execution. Steps run in definition order, in the invoker's working directory, and with the invoker's environment.
- A streaming transcript. Each step prints its id, then that step's stdout and stderr as the process writes them. A successful run exits 0 and prints no status block.
- Fail-fast results. A non-zero exit, or a command that cannot be started, marks that step and the pipeline failed. Later steps stay unrun. The process exits 1 and prints one error line naming the pipeline, the step, and the exit code or start error.
- Load and validation errors before any command runs: unreadable path, invalid YAML, missing or empty pipeline name, missing or empty steps, missing or empty step id, duplicate step ids, missing or empty `run`, and a missing or unsupported schema version. Any other invocation prints `usage: truss run [<path>]` and exits 1.
- Structured run data for each executed step: status (`succeeded` or `failed`), exit code when a command ran, captured output, and failure context when a command did not start.
- MIT license.
- Specification-driven workflow for contributors: project constitution (1.1.0, including test-first development), product glossary in `docs/GLOSSARY.md`, and the host pipeline engine specification, plan, contracts, and tasks under `specs/001-host-pipeline-engine/`.

[Unreleased]: https://github.com/afri-bit/truss/commits/main
