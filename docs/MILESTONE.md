# Release milestone

This file is the release scope for TRUSS. Feature specifications say how one capability behaves. This file says which capabilities belong in the current release and which wait.

Agents and contributors read it before choosing or expanding a feature. `develop` is the integration branch. New work uses the branch names in `.specify/memory/constitution.md`.

## v0.1.0: the MVP

v0.1.0 is the first tagged release and the first version worth using every day on your own
machine: write a pipeline file, run it by hand, or hand it to the daemon and let it run on a
schedule, and watch all of it from the terminal or from TRUSS Desktop.

Do not tag v0.1.0 until every item in this section has a specification and an implementation
that matches it, and the checklist below is done. `go.mod`, `cmd/truss` and Desktop already say
`0.1.0`, and the changelog has its `## [0.1.0]` section; its date must be the day the tag is pushed.

### In this release

| Capability | What it means | Spec | State |
|---|---|---|---|
| Local runs | `truss run <name>` executes a pipeline of the project (or any file by path) on the host shell, fail-fast | `001-host-pipeline-engine` | Implemented |
| Run log | Timestamped, readable log; `truss help`, `truss version` | `002-local-run-log` | Implemented |
| Step dependencies | `needs` between steps | `003-step-dependencies` | Implemented |
| Step concurrency | `pipeline.concurrency`, strict fields, stoppable process trees | `004-step-concurrency` | Implemented |
| Jobs, schedules, step settings | `jobs`, `on.schedule`, `env`, `dir`, `timeout`, `overlap`, `truss validate` | `005-jobs-schedules-and-step-settings` | Implemented |
| Daemon | Background service, registry, cron scheduling with missed-run policy, run queue and history, HTTP API, CLI commands | `006-daemon-registry-and-scheduling` | Implemented |
| Run tracking and queue | Queued runs survive a daemon restart; `truss run --track` reports a local run to the daemon (`local` trigger) | `010-run-tracking-and-queue` | Implemented |
| Secrets | `truss secret`, `secrets` in the pipeline file, redaction of values in step output | `012-secrets` | Implemented |
| TRUSS Desktop | Dashboard, pipeline control, run view with graph and live log, file picker, browser tests | `007-truss-desktop`, `013-desktop-polish`, `014-desktop-redesign` | Implemented |
| Resource figures | `GET /metrics`: host, daemon and step CPU and memory on Linux, macOS and Windows; step process ids; Desktop shows them (added by the maintainer, 2026-11-13) | `015-resource-metrics` | Implemented |
| Retries and conditions | `retries`, `retry_delay`, `continue-on-error`, `if` on steps, `if` on jobs | `009-retries-and-conditions` | Implemented |
| Project layout | `truss.yaml` is project metadata; pipelines are files in `.truss/flows/`; generated files in `.truss/state/` (added by the maintainer, 2026-11-15) | `016-project-layout` | Implemented |
| Documentation site | A site with the docs (search, sidebar, light and dark) and a home page, built from `docs/` (added by the maintainer, 2026-11-16) | `017-docs-site` | Implemented (the first deploy needs the repository's Pages setting) |

**Operating systems.** Linux, macOS and Windows. A `run` string is a command for that host's
shell; v0.1.0 does not promise that one file runs unchanged on every shell.

### Release checklist

What is left before the tag. Items marked "you" cannot be done from code.

- [ ] The pull requests for the project layout, the repository's own pipelines and the docs site are merged into `develop`.
- [ ] CI is green on Linux, macOS and Windows for them. The daemon, Desktop and the project layout have only run on Linux and macOS locally; Windows and the Desktop browser tests (Linux only, by design) get their first proof here.
- [ ] You: try `truss daemon install` for real on each OS you support. It is verified by generation and fakes only ([spec 011](../specs/011-daemon-start-at-login/spec.md)).
- [ ] You: Settings > Pages > Source: "GitHub Actions", so the docs site deploys from `main`.
- [ ] `release/0.1.0` is merged into `main` through a pull request, then the tag is pushed (see "Releasing" in the README).
- [ ] `main` is merged back into `develop`.

### Not in v0.1.0

- Container execution and remote workers
- Matrix builds, caches, artifacts, step outputs in conditions
- Encrypted or OS-keychain secret storage, per-pipeline secret scope
- Triggers other than cron and manual (file changes, webhooks)
- A packaged desktop window, tray icon and notifications
- SDKs and CI-provider plugins
- A pipeline file that is portable across host shells

## How to change this file

Change this file when the release scope changes. Do not treat a chat or a local note as the milestone. A new capability inside v0.1.0 gets its own `specs/<feature>/spec.md` in the same pull request as its code.
