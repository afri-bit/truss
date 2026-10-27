# Release milestone

This file is the release scope for TRUSS. Feature specifications say how one capability behaves. This file says which capabilities belong in the current release and which wait.

Agents and contributors read it before choosing or expanding a feature. `develop` is the integration branch. New work uses the branch names in `.specify/memory/constitution.md`.

## v0.1.0

v0.1.0 is the first tagged release. It is the smallest version a developer can use on Linux, macOS, and Windows: one pipeline file, one local command, a log that explains the run, and a pipeline that can wait on other steps and run independent steps together.

Do not tag v0.1.0 until every item in this section has a specification and an implementation that matches it. `go.mod` already says `// version: 0.1.0`. The changelog stays under `[Unreleased]` until the tag.

### In this release

**Workflow.** `truss.yaml` in the working directory is the pipeline definition. `truss run` loads that file, or an explicit path. `.truss/run.log` stores the latest run log and is replaced by the next finished run. TRUSS does not edit ignore files and does not keep run history.

**Run log.** The log is readable while the run proceeds and is the same text saved afterward.

- The setup section says the environment was captured and is ready, then shows the operating system, architecture, host shell, and working directory.
- Every line TRUSS writes has a local timestamp, `YYYY-MM-DD HH:MM:SS`.
- Each started step shows when it started, its output, when it finished, how long it took, and its exit code. A step that cannot start shows a diagnostic and no exit code.
- The summary shows the pipeline status, how many steps started, the run duration, and when the run finished.
- The setup section and summary do not list environment variable names, environment variable values, or secrets. There is no secret store in this release.

**Engine.** Parsing, validation, planning, and host execution stay independent of the CLI. Steps run on the host shell. A failure fails the whole run: in-flight work stops, and steps that have not started do not run.

**CLI.** `truss run` starts a run. `truss help` and `truss version` answer without starting one. Any other invocation prints usage and does not start a run.

**Operating systems.** Linux, macOS, and Windows are the supported hosts. A `run` string is a command for that host's shell. v0.1.0 does not promise that one pipeline file runs unchanged on every shell.

**Pipeline file.** Schema version `"1"` gains step dependencies and concurrency before the tag, in their own specification:

- `needs` lists step ids that must succeed before a step starts. A time delay is an ordinary `run` command, not a schema field.
- Concurrency defaults to 1, so a file with no `needs` stays sequential. A higher concurrency runs steps whose needs are already met.
- A failure still fails the whole run. Independent branches do not continue.
- Unknown pipeline fields are rejected once that specification lands, so a misspelled `needs` cannot run in file order.

### Already specified

| Feature | Spec | State |
|---|---|---|
| Sequential host engine | `specs/001-host-pipeline-engine/` | Implemented |
| Local run log, help, and version | `specs/002-local-run-log/` | Implemented |
| Step `needs`, still one step at a time | `specs/003-step-dependencies/` | Implemented |

The next specification is concurrency: ready steps run together when the pipeline raises the limit above 1. Do not start that code before its spec, plan, and tasks exist. Until then, execution stays sequential even when steps declare `needs`.

### Not in v0.1.0

These stay on the roadmap until a later release:

- Daemon, API, and SDKs
- Container execution and remote workers
- Matrix builds, cache, artifacts, and a secret store
- CI-provider plugins and dashboards
- A pipeline file that is portable across host shells
- Per-step environment, working directory, timeouts, and retries
- Cancellation as a reported status
- Named deployment environments

## How to change this file

Change this file when the release scope changes. Do not treat a chat or a local note as the milestone. A new capability inside v0.1.0 still gets its own `specs/<feature>/` before implementation.
