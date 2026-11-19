# Release milestone

This file is the release scope for TRUSS. Feature specifications say how one capability behaves. This file says which capabilities belong in the current release and which wait.

Agents and contributors read it before choosing or expanding a feature. `main` is the default and integration branch. New work uses the branch names in `.specify/memory/constitution.md`.

## v0.1.0: the MVP

v0.1.0 was tagged on 2026-11-16. It is the first tagged release and the first version worth using every day on your own
machine: write a pipeline file, run it by hand, or hand it to the daemon and let it run on a
schedule, and watch all of it from the terminal or from TRUSS Desktop.

Every item in this section has a specification and an implementation that matches it. `go.mod`,
`cmd/truss` and Desktop said `0.1.0` at the tag, and the changelog has its `## [0.1.0]` section.

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

The v0.1.0 checklist is done: the maintainer considers every item below complete (2026-11-19). It
is kept as the record of what the release needed.

- Daemon and Desktop are verified on Linux only. macOS and Windows build; their first proof is CI.
- TRUSS Desktop browser tests run on Linux in CI only.
- Start at login is automated by `truss daemon install` ([spec 011](../specs/011-daemon-start-at-login/spec.md)) but verified by generation and fakes only; its first real run on each OS is a maintainer try-out.
- [x] The pull requests for the project layout, the repository's own pipelines and the docs site are merged into `main`.
- [x] CI is green on Linux, macOS and Windows for them. The daemon, Desktop and the project layout have only run on Linux and macOS locally; Windows and the Desktop browser tests (Linux only, by design) get their first proof here.
- [x] You: try `truss daemon install` for real on each OS you support. It is verified by generation and fakes only ([spec 011](../specs/011-daemon-start-at-login/spec.md)).
- [x] You: Settings > Pages > Source: "GitHub Actions", so the docs site deploys from `main`.
- [x] `release/0.1.0` is merged into `main` through a pull request, then the tag is pushed (see "Releasing" in [CONTRIBUTING.md](../CONTRIBUTING.md)).

### Not in v0.1.0

- Container execution and remote workers
- Matrix builds, caches, artifacts, step outputs in conditions
- Encrypted or OS-keychain secret storage, per-pipeline secret scope
- Triggers other than cron and manual (file changes, webhooks)
- A packaged desktop window, tray icon and notifications
- SDKs and CI-provider plugins
- A pipeline file that is portable across host shells

## v0.2.0: debug a flow, and a richer terminal

v0.2.0 adds what makes TRUSS useful when a flow fails: look at one step, see everything it needs,
and run just that step with inputs of your choice. Alongside it come the changes made after
v0.1.0 was tagged (the table in v0.1.0 listed three of them before; they belong here).

### In this release

| Capability | What it means | Spec | State |
|---|---|---|---|
| Debug a step | `truss flows show <flow> <step>` shows a step's command, its neighbours and the environment it can use, with values (secrets as set or missing); `truss run <flow> --step <id>` (`--with-needs`, `--from`, `--env`) runs part of a flow without replacing the saved run log. The headline of the release (added by the maintainer, 2026-11-19) | `023-step-debugging` | Implemented |
| Flows follow their files | A registered file is the source of truth: the daemon notices an edit, shows a renamed flow under its new name and keeps its history (added by the maintainer, 2026-11-16) | `019-flows-follow-their-files` | Implemented |
| A readable run log | The run log has headings for the setup, each step and the summary, and TRUSS Desktop shows it as one card per step (added by the maintainer, 2026-11-16) | `020-readable-run-log` | Implemented |
| A richer command line | Read-only `status`, `dashboard`, `flows`, `project(s)`, `schedules`, `runs show`, `daemon logs`, `env`, `help <command>` and `--json`; ASCII screens (added by the maintainer, 2026-11-17) | `022-cli-enrichment` | Implemented |
| Desktop corrections | Flows page naming, Re-run, the Resources chart (added by the maintainer, 2026-11-16) | `018-desktop-corrections` | Implemented |
| Flows view toggle | The Flows page as cards or as a list (added by the maintainer, 2026-11-16) | `021-flows-view-toggle` | Implemented |

### Release checklist

- [x] The pull request of `023-step-debugging` is merged into `main`.
- [ ] CI is green on Linux, macOS and Windows. The step view and the debug run have only run on macOS locally; Windows gets its first proof here.
- [x] `release/0.2.0`: `0.2.0-beta` becomes `0.2.0` in `go.mod`, `cmd/truss` and `internal/webui/VERSION`; `[Unreleased]` becomes `[0.2.0]` with the date; the release check passes.
- [ ] `release/0.2.0` is merged into `main` through a pull request, then the maintainer pushes the tag.

### Not in v0.2.0

An interactive shell in a step's environment, step-by-step stepping, `--dry-run`, resuming from
the last failure, saving a debug log, debug runs through the daemon, event triggers (hooks), a daemon reachable beyond localhost, per-step runners (container,
other shells) and AI integration (an MCP server). Each needs its own decision and spec; see the
roadmap below.

## Roadmap

What comes after v0.2.0, roughly in this order. Scope and order may change: a release holds only
what its section above lists. A box is ticked when the work is merged and has a spec. Each item
needs its own `specs/<feature>/spec.md`; the ones that change a contract or the execution of a
process are tier 2 and get an ADR.

**Done**

- [x] v0.1.0: local runs, the daemon and schedules, secrets, TRUSS Desktop, the project layout
- [x] v0.2.0: inspect a step and run part of a flow, the layered command line, the readable run log, flows that follow their file, the Flows list view, the docs site

**v0.3.0: debug further, and a first step towards agents**

- [ ] **Debug, phase 2.** An interactive shell in a step's exact environment (`--shell`), stepping through a flow (`--break`), `--dry-run`, resume from the last failure, save a debug log (`--log`), the last result of a step in `flows show`, and a way to try a step that runs only after a failure (`if: failure()`). `--shell` starts a process the user did not declare as a step, so it needs a line in the execution-safety rules and an ADR.
- [ ] **`requires:` on a step.** The YAML declares what the step needs from outside (variables, files), so inspect can flag what is missing without reading the script the step runs. A pipeline schema change.
- [ ] **A shell per step, and step outputs.** `shell: bash | sh | pwsh | cmd` on a step, and `TRUSS_OUTPUT` to pass data between steps. It also covers the "pipeline file portable across host shells" item.
- [ ] **Hooks, local first.** A flow starts when another flow ends (`on.run`) or when files change (`on.file`). Both need no network.
- [ ] **`truss mcp`.** An MCP server over stdio, a thin client of the daemon API. It runs registered flows only (never an arbitrary command) and is read-only by default, so an agent can inspect a failed step and rerun just that step.

**Later**

- [ ] **Per-step platforms.** `runner: docker` for a step, using the docker CLI and the project folder mounted as the workspace (no new dependency, no image holding every tool), cancellation by container name. Then remote workers, so a flow keeps one environment while each step runs where its tool lives. Moving the environment between steps means the workspace, the variables and the step outputs; a general file sync waits for remote workers.
- [ ] **Hooks from outside.** Webhooks as triggers.
- [ ] **A daemon reachable beyond localhost.** TLS (or a documented SSH tunnel), scoped tokens (read-only against control), and an explicit `--allow-remote`. The daemon runs shell commands for whoever holds its token, so this needs an ADR and a security review, and comes before any remote worker.
- [ ] **Agents.** Recipes for running an agent as a step, an approval gate, then SDKs. TRUSS runs agents and is called by them; it is not an LLM framework.
- [ ] **Desktop.** A packaged window with a tray icon, notifications and start at login.
- [ ] **Pipelines.** Matrix builds and artifacts.
- [ ] **Ecosystem.** SDKs and CI-provider bridges, so the same file runs in CI.

## How to change this file

Change this file when the release scope changes. Do not treat a chat or a local note as the milestone. A new capability inside the current release gets its own `specs/<feature>/spec.md` in the same pull request as its code.
