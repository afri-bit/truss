# Changelog

All notable changes to TRUSS are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Dates below are when a release was made. 0.1.0 is the first release.

## [Unreleased]

### Changed

- The daemon and TRUSS Desktop follow a registered file when its `pipeline.name` changes: the flow shows the new name, keeps all of its runs, and answers to both names in commands and the API. A registration keeps the name it was registered under for its history, schedules and data. The API adds the optional fields `registered_name` (on a pipeline) and `registered_pipeline` (on a run) while they differ from `name` and `pipeline`; `name` and `pipeline` now carry the name from the file. A name another flow holds, or one chosen with `--name`, is never replaced; a registration made earlier with `--name` follows the file unless it is registered again with `--name`. A file registered again without `--name` is no longer registered twice. Views no longer lag an edit by up to a second (specs/019-flows-follow-their-files, ADR 0017).
- TRUSS Desktop: the sidebar item, page heading, breadcrumbs and *Jump to…* entry and group that list registered pipelines now say *Flows* (the addresses and the API are unchanged), and the run page's *Run again* button is now *Re-run* (specs/018-desktop-corrections).

### Fixed

- TRUSS Desktop: on a running run's Resources chart the memory line no longer leaves the chart and draws over the panel title. Memory is scaled into the CPU axis and the legend says *mem, scaled* (specs/018-desktop-corrections).

## [0.1.0] - 2026-11-16

The first release: run a pipeline file by hand, or let the daemon run it on a schedule, and watch it from the terminal or TRUSS Desktop, on Linux, macOS and Windows.

### Changed

- Project layout (breaking against earlier development builds; specs/016-project-layout, ADR 0015). `truss.yaml` is now project metadata only: `version`, `project.name` and `project.description`. The project's `truss.yaml` (in the working directory) with `steps` or `jobs` is rejected; a `truss.yaml` in another folder, given by path, runs as any pipeline file. Pipelines are files in `.truss/flows/` (`.yaml` or `.yml`), run by name with `truss run <name>`; `truss run <path>` still runs any file. `pipeline.name` is optional and defaults to the file name. A run's log and data move to `.truss/state/<name>/` (`run.log`, `data/`, `data/cache/`), and TRUSS writes `.truss/.gitignore` so git ignores only that folder. Older `.truss/run.log` and `.truss/data` are ignored. `truss register --all` registers every pipeline in a project. Pipeline files are capped at 1 MiB. A flow registered in a project runs its steps in the project root. The error `prepare data folder` is now `prepare state folder`. After a successful `register --all` (every flow registered, none failed) the CLI prints `registered N pipelines (M with schedules)`, or `registered 1 pipeline (M with schedules)` for one. No summary is printed when a flow failed.
- License: TRUSS is now released under the Apache License 2.0 (was MIT). Nothing was tagged under MIT. A `NOTICE` file ships in every release archive.
- Release process: the release check also compares the tag with `truss version` and with TRUSS Desktop's own version (`internal/webui/VERSION`), and accepts only the Keep a Changelog headings. Branches must start with `bugfix/`, `feature/`, `hotfix/`, `release/`, `task/` or `chore/`, and commits must be authored by `afri-bit`; CI and the agent hooks enforce both (ADR 0014, constitution 2.1.0).
- Queued runs survive a daemon restart. They stay `queued` when the daemon stops and start in request order when it starts again, instead of being cancelled. A queued run whose pipeline can no longer be loaded is closed as `failed`. Runs that were running are cancelled on a graceful stop, or closed as `failed` after a crash, as before.
- `truss help` and usage errors print the full command list. The first usage line is now `usage: truss <command> [arguments]`.
- Project process: `golangci-lint` (with `gosec`) is now a blocking CI check, also for Windows- and macOS-only files, and `scripts/verify.sh` runs the same quality gate locally.
- Project process: constitution 2.0.0. A specification travels with the code instead of gating it, and tests ship with the behavior (see `docs/decisions/0005-specs-record-they-do-not-gate.md`).
- Require Go 1.27.3. Go 1.22.5's standard library is affected by GO-2025-3956 (`os/exec` LookPath) and GO-2025-3750 (`os` and `syscall`), which the vulnerability scan reports through the host executor and CLI.
- Processes a Step starts are ended when that Step's shell exits, so a Step can no longer leave a background process running for a later Step.
- A Step ended by a signal is logged with a diagnostic and no Exit Code instead of `exit -1`.
- On Linux and macOS each Step runs in its own process group so TRUSS can end it with everything it started. A Step that waits for terminal input (a password prompt) is stopped by the terminal until `truss` is interrupted. Interrupting `truss` (Ctrl-C or SIGTERM) now ends the running Steps and exits with status 1.
- Unknown fields in a pipeline definition are now rejected, at the top level, under `pipeline`, and in a Step. Before, they were ignored. The error names the field and where it appears.

### Added

- **Documentation site.** The docs are now also a website with search, a sidebar and light and dark schemes, and a home page that shows a pipeline being built and run, schedules with a catch-up run, and TRUSS Desktop. It is built from `docs/` by `site/` (Astro and Starlight, pinned; not part of the product). A new *Getting started* page takes you from install to a scheduled pipeline. The brand is amber on graphite, the colours of TRUSS Desktop (specs/017-docs-site, ADR 0016). It is published to GitHub Pages from `main` by CI.
- **TRUSS Desktop redesign.** Collapsible sidebar, header with breadcrumbs, *Jump to…* and a settings menu, light and dark themes, a Monitor page (health checks, CPU, memory, cores, processes) and a Settings page. The run page shows progress, the running steps, a stage-by-stage step graph with the running step highlighted, and a live log with line numbers, find, wrap and follow. Pipeline cards keep every element in a fixed position (specs/014-desktop-redesign).
- **Resource figures.** `GET /api/v1/metrics` reports the machine's CPU, per-core load, memory and load average, the daemon's own process, and each running step's processes, with ten minutes of history. Steps carry their process id (`pid`). Linux, macOS and Windows. TRUSS Desktop shows these instead of simulated figures (specs/015-resource-metrics).
- **`truss desktop --demo`** shows TRUSS Desktop on moving sample data without a daemon, for trying it or reviewing its design. Off by default; Ctrl-C stops it.
- **Retries.** A step may set `retries` (0 to 100) and `retry_delay`. A failed attempt is run again; each attempt is logged with its own `started` and `finished` lines, and a `step <id> retry <n> of <total> in <delay>` line comes between them. The step's result is its last attempt (specs/009-retries-and-conditions).
- **`continue-on-error: true`** on a step. Its failure is logged and shown but does not fail the run or stop other steps. Steps that need it are skipped unless their `if` says otherwise.
- **Conditions.** `if` on a step or a job takes a small expression: `always()`, `success()`, `failure()`, `==`, `!=`, `&&`, `||`, `!`, parentheses, quoted text and `env.NAME`. A skipped step is logged as `step <id> skipped because <reason>` and shown as skipped. After a failure only steps whose condition calls `always()` or `failure()` start. Definitions without these fields behave exactly as before; an older TRUSS rejects them as unknown fields.
- The daemon API reports `attempts`, `continued_on_error` and `reason` on a step, only where they apply. TRUSS Desktop shows attempts, skip reasons and tolerated failures.
- **Start at login.** `truss daemon install` makes the daemon start at login (a systemd user unit on Linux, a launchd agent on macOS, a Task Scheduler task on Windows; no root or administrator rights) and `truss daemon uninstall` removes exactly what it created. `truss daemon status` shows whether login start is installed. Install refuses a binary in a temporary directory. See `specs/011-daemon-start-at-login`.
- **Tracked local runs.** `truss run --track` still executes in your shell with your environment, and also reports the run to the daemon: it shows up in `truss runs` and TRUSS Desktop with the trigger `local`, its steps and its log. It needs a running daemon and fails with the usual hint before running anything if there is none; if the daemon goes away mid-run, the run finishes normally with one warning. Cancelling a tracked run in the daemon or Desktop stops the local run. Desktop shows a `local` badge. New API v1 endpoints, additive: `POST /runs`, `POST /runs/{id}/steps`, `POST /runs/{id}/log?offset=`, `POST /runs/{id}/finish` (see `docs/decisions/0007-tracked-local-runs.md`).
- **Secrets.** `truss secret set|list|remove` keeps credentials in `secrets.json` in the TRUSS home (private file, not encrypted; `set` reads standard input or a hidden prompt, never an argument). A pipeline, job or step lists `secrets: [NAME]` and receives them as environment variables. A missing secret fails the run before any step starts. Stored values in step output are replaced by `***` in the run log, saved logs, API and Desktop; encoded or transformed values are not caught (specs/012-secrets, ADR 0006).
- **The daemon.** `truss daemon start`, `stop`, `status` and `run` manage a background service that listens on `127.0.0.1:7878`. It keeps its state under `~/.truss` (or `TRUSS_HOME`) and protects its HTTP API (`/api/v1`) with a token only your user can read.
- **Registered pipelines.** `truss register [<path>] [--name <name>]`, `unregister`, `pipelines`, `pause` and `resume`. The daemon stores the file's path, reads it again before every run, and notices edits.
- **Daemon runs.** `truss trigger <name> [--follow]`, `runs`, `logs <run-id> [--follow]` and `cancel`. Runs queue up; `--max-runs` limits how many execute at once (default 4). The newest 200 runs and their logs are kept.
- **Schedules.** `on.schedule` lists cron triggers with an optional `timezone` and a `missed` policy. A time that passed while the machine was off or asleep is skipped and recorded by default; `missed: run` starts one catch-up run.
- **Jobs.** `jobs` groups ordered steps under a name. Jobs wait for the jobs in their `needs`. In logs a job's step is named `job/step`.
- **Step settings.** `env` at pipeline, job and step level; `dir` and `timeout` per step. A step that exceeds its timeout fails with `timed out after <duration>`.
- **Workdir.** `pipeline.workdir` sets the directory steps start in, relative to where the run starts or absolute. A job's or step's `dir` still wins. A missing directory fails the step with a diagnostic and no exit code. Additive: existing files keep their meaning.
- **Data and cache folders.** Steps get `TRUSS_DATA_DIR` and `TRUSS_CACHE_DIR`: `~/.truss/pipelines/<name>/` and its `cache/` for daemon runs (kept when you unregister), `.truss/data` and `.truss/data/cache` for `truss run`. `truss cache clear <name>` empties a pipeline's cache.
- **Overlap policy.** `pipeline.overlap` is `queue` (default), `skip` or `parallel`.
- **`truss validate [<path>]`** checks a definition and prints its schedules and steps without running anything.
- **TRUSS Desktop.** `truss desktop` opens a dashboard served by the daemon: counters, active and upcoming runs, pipeline controls, and a run page with the step graph and a live log.
- **File picker for registering.** TRUSS Desktop's register form has a *Browse…* button. It uses the new `GET /api/v1/fs` endpoint, which lists directories and `*.yaml`/`*.yml` names (never content) starting at your home directory (specs/013-desktop-polish, ADR 0010).
- Browser tests for TRUSS Desktop in `tests/ui/` (Node and Playwright, run by a new `desktop-ui` CI job). Playwright is not a dependency of the product.
- TRUSS Desktop polish: a "Daemon unreachable" page, an error page with *Try again*, empty-log messages, a register form that keeps what you typed across refreshes, keyboard focus kept across refreshes, and keyboard access to run rows, step rows and graph nodes.
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

### Security

- `truss run` no longer writes through symbolic links a repository can carry in `.truss`. A link planted at `.truss/run.log.tmp` used to receive the run log, overwriting a file outside the folder. A `.truss`, `.truss/data` or `.truss/data/cache` that is a link is now refused before any step starts; before, the run log, data and cache went into the folder the link pointed to (specs/002-local-run-log FR-018).

[Unreleased]: https://github.com/afri-bit/truss/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/afri-bit/truss/releases/tag/v0.1.0
