# Release milestone

This file is the release scope for TRUSS. Feature specifications say how one capability behaves. This file says which capabilities belong in the current release and which wait.

Agents and contributors read it before choosing or expanding a feature. `develop` is the integration branch. New work uses the branch names in `.specify/memory/constitution.md`.

## v0.1.0: the MVP

v0.1.0 is the first tagged release and the first version worth using every day on your own
machine: write a pipeline file, run it by hand, or hand it to the daemon and let it run on a
schedule, and watch all of it from the terminal or from TRUSS Desktop.

Do not tag v0.1.0 until every item in this section has a specification and an implementation
that matches it. `go.mod` already says `// version: 0.1.0`. The changelog stays under
`[Unreleased]` until the tag.

### In this release

| Capability | What it means | Spec | State |
|---|---|---|---|
| Local runs | `truss run` executes `truss.yaml` on the host shell, fail-fast | `001-host-pipeline-engine` | Implemented |
| Run log | Timestamped, readable log; `truss help`, `truss version` | `002-local-run-log` | Implemented |
| Step dependencies | `needs` between steps | `003-step-dependencies` | Implemented |
| Step concurrency | `pipeline.concurrency`, strict fields, stoppable process trees | `004-step-concurrency` | Implemented |
| Jobs, schedules, step settings | `jobs`, `on.schedule`, `env`, `dir`, `timeout`, `overlap`, `truss validate` | `005-jobs-schedules-and-step-settings` | Implemented |
| Daemon | Background service, registry, cron scheduling with missed-run policy, run queue and history, HTTP API, CLI commands | `006-daemon-registry-and-scheduling` | Implemented |
| TRUSS Desktop | Dashboard, pipeline control, run view with graph and live log, file picker, browser tests | `007-truss-desktop`, `013-desktop-polish` | Implemented |

**Operating systems.** Linux, macOS and Windows. A `run` string is a command for that host's
shell; v0.1.0 does not promise that one file runs unchanged on every shell.

### Before the tag

These are known gaps inside the release scope:

- Daemon and Desktop are verified on Linux only. macOS and Windows build; their first proof is CI.
- TRUSS Desktop browser tests run on Linux in CI only.
- Start at login is documented, not automated.
- `docs/agents.md` and the `/feature` skill still describe the pre-2.0.0 process.

### Not in v0.1.0

- Container execution and remote workers
- Matrix builds, conditions, retries, caches, artifacts, a secret store
- Triggers other than cron and manual (file changes, webhooks)
- A packaged desktop window, tray icon and notifications
- SDKs and CI-provider plugins
- A pipeline file that is portable across host shells

## How to change this file

Change this file when the release scope changes. Do not treat a chat or a local note as the milestone. A new capability inside v0.1.0 gets its own `specs/<feature>/spec.md` in the same pull request as its code.
