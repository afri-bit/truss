<h1 align="center">TRUSS</h1>

<p align="center">
  <img src="docs/images/logo.svg" alt="TRUSS logo" width="128" height="128">
</p>

<p align="center">
  <strong>Pipelines that live on your machine.</strong><br>
  Write one YAML file, run it by hand or on a schedule,<br>
  and watch it from the terminal or from TRUSS Desktop.
</p>

<p align="center">
  <a href="https://github.com/afri-bit/truss/actions/workflows/ci.yml"><img src="https://github.com/afri-bit/truss/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.27.3-00ADD8?logo=go" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License: Apache 2.0"></a>
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="docs/pipeline-reference.md">Pipeline file</a> ·
  <a href="docs/cli.md">Commands</a> ·
  <a href="docs/daemon.md">Daemon</a> ·
  <a href="docs/desktop.md">Desktop</a> ·
  <a href="docs/architecture.md">Architecture</a>
</p>

> **Status:** early development. The first release, [v0.1.0](docs/MILESTONE.md), is not tagged yet. The pipeline file, the CLI and the daemon API may still change until then.

<p align="center">
  <img src="docs/images/desktop-dashboard.png" alt="TRUSS Desktop dashboard" width="900">
</p>

## Contents

- [Why TRUSS](#why-truss)
- [What you get](#what-you-get)
- [Quick start](#quick-start)
- [Three ways to use TRUSS](#three-ways-to-use-truss)
- [A pipeline file](#a-pipeline-file)
- [Commands](#commands)
- [How it fits together](#how-it-fits-together)
- [Documentation](#documentation)
- [Roadmap](#roadmap)
- [Development](#development)
- [License](#license)

## Why TRUSS

Pipelines are software, but they are rarely developed like software.

- **Slow feedback.** You commit and push, then wait in a CI queue just to find out whether a single command works.
- **Locked to one platform.** A pipeline written for GitHub Actions, GitLab CI, Jenkins, or Azure Pipelines only runs there. Moving platforms means rewriting it.
- **Hard to debug.** Failures happen on a remote runner you can't step into, with an environment you can't easily reproduce.
- **Implicit behavior.** It is often unclear what runs, in what order, with which environment, and why it failed.

TRUSS treats a pipeline as an executable artifact rather than platform configuration. The same definition runs locally while you work on it and unchanged wherever it is deployed.

```mermaid
flowchart LR
    subgraph today["Today: CI-driven loop"]
        direction LR
        a1[Edit] --> a2[Commit] --> a3[Push] --> a4[Wait for CI] --> a5[Discover failure]
        a5 -.->|repeat| a1
    end
    subgraph withtruss["With TRUSS: local loop"]
        direction LR
        b1[Edit] --> b2[Run locally] --> b3[Observe] --> b4[Fix]
        b4 -.->|repeat| b1
    end
```

## What you get

- **Local first.** `truss run` executes a pipeline right where you are. No server, no account, no push.
- **Jobs and dependencies.** Group steps into jobs, let jobs wait for each other, run independent work in parallel.
- **A daemon, like Docker's.** `truss daemon start` gives you a queue, a history and one API for every client.
- **Cron that understands laptops.** Schedules live in the pipeline file. A job planned for 09:00 does not fire at 11:00 because you opened the lid late, unless you ask for a catch-up run.
- **Register what you want watched.** Your files stay in your projects; the daemon only remembers where they are and re-reads them when they change.
- **Concurrency you control.** Steps per run, runs per machine, and whether a pipeline may overlap itself.
- **TRUSS Desktop.** A dashboard, pipeline controls, step graphs and live logs, built into the same binary.
- **One small binary.** Go standard library plus a YAML parser. Linux, macOS and Windows.

## Quick start

### Install

With Go 1.27.3 or newer:

```bash
go install github.com/afri-bit/truss/cmd/truss@latest
```

Or build from source:

```bash
git clone https://github.com/afri-bit/truss.git
cd truss
go build -o bin/truss ./cmd/truss
```

Prebuilt binaries will be published on [GitHub Releases](https://github.com/afri-bit/truss/releases) with the first tagged version.

### 1. Run a pipeline by hand

Create `truss.yaml`:

```yaml
version: "1"

pipeline:
  name: hello

steps:
  - id: greet
    run: echo "Hello from TRUSS"
```

```console
$ truss run
2026-11-12 09:00:00 setup environment ready
2026-11-12 09:00:00 setup os linux
2026-11-12 09:00:00 setup arch amd64
2026-11-12 09:00:00 setup shell /bin/sh
2026-11-12 09:00:00 setup directory /home/me/project
2026-11-12 09:00:00 step greet started
2026-11-12 09:00:00 step greet | Hello from TRUSS
2026-11-12 09:00:00 step greet finished duration 0.003s exit 0
2026-11-12 09:00:00 summary succeeded steps 1 duration 0.005s
```

### 2. Let the daemon run it

```console
$ truss daemon start
daemon started (pid 4242, 127.0.0.1:7878)

$ truss register
registered hello (/home/me/project/truss.yaml)

$ truss trigger hello --follow
...
2026-11-12 09:01:10 summary succeeded steps 1 duration 0.005s

$ truss runs
RUN                     PIPELINE  STATUS     TRIGGER  CREATED              DURATION  NOTE
20261112-090110-a3f9c1  hello     succeeded  manual   2026-11-12 09:01:10  0s
```

The daemon does not start by itself after a reboot. To have your schedules keep working, run
`truss daemon install` once; it starts the daemon at every login and needs no admin rights
(`truss daemon uninstall` takes it back). Details in [the daemon docs](docs/daemon.md#starting-at-login).

### 3. Put it on a schedule

Add a trigger to the file and save. The daemon picks the change up by itself.

```yaml
on:
  schedule:
    - cron: "0 9 * * 1-5"   # weekdays at 09:00
```

```console
$ truss pipelines
NAME   STATE  NEXT RUN             LAST RUN                       FILE
hello  ready  2026-11-13 09:00:00  succeeded 2026-11-12 09:01:10  /home/me/project/truss.yaml
```

### 4. Open TRUSS Desktop

```console
$ truss desktop
```

## Three ways to use TRUSS

| | You start | You use | Tracked by the daemon? |
|---|---|---|---|
| **1. Alone** | nothing | `truss run`, `truss validate` | No. The log is `.truss/run.log` in the project. Add `--track` (needs the daemon) to also record the run. |
| **2. With the daemon** | `truss daemon start` | the CLI: `register`, `trigger`, `runs`, `logs`, `cancel`, ... | Yes: schedules, queue, history. |
| **3. With the daemon and Desktop** | `truss daemon start`, then `truss desktop` | the same runs, seen and controlled in the browser | Yes, same state as path 2. |

The daemon is the server, like Jenkins or Docker's engine. It is always started on its own
and stays in the background. Desktop and the CLI are clients: without a daemon Desktop shows
nothing, and neither client starts or stops it.

Path 1 is deliberately separate. `truss run` executes in your shell with your environment, needs
no server, and behaves the same in CI. If you want a run to appear in the history and in Desktop,
either register the file and use `truss trigger` (the daemon runs it, with the daemon's environment),
or keep running in your shell and add `--track`: `truss run --track` still uses your environment and
only reports to the daemon, which shows it as a `local` run.

Pipelines are written in files, with your editor. Desktop watches and controls them; it does not edit them.

## A pipeline file

```yaml
version: "1"

pipeline:
  name: nightly
  concurrency: 3            # steps at once
  overlap: skip             # never run on top of itself

on:
  schedule:
    - cron: "30 1 * * *"
      missed: run           # catch up once if the machine was off

env:
  STAGE: nightly

jobs:
  lint:
    steps:
      - id: vet
        run: go vet ./...
  test:
    steps:
      - id: unit
        run: go test ./...
        timeout: 10m
  release:
    needs: [lint, test]     # waits for both
    steps:
      - id: build
        run: go build -o dist/app ./cmd/app
      - id: publish
        run: ./scripts/publish.sh
        env:
          CHANNEL: nightly
```

`lint` and `test` run side by side; `release` starts when both succeeded. The first failed step
fails the run and stops the rest, unless you opt into retries, `continue-on-error` or `if`
conditions. Every field is described in the
[pipeline file reference](docs/pipeline-reference.md).

## Commands

| | Command | |
|---|---|---|
| **Run here** | `truss run [<path>] [--track]` | Run a file now; `--track` also reports it to the daemon |
| | `truss validate [<path>]` | Check a file and show its steps |
| **Secrets** | `truss secret set <name>` \| `list` \| `remove <name>` | Store credentials for `secrets:` in a file (value from stdin) |
| **Daemon** | `truss daemon start` \| `stop` \| `status` | Manage the background service |
| | `truss daemon install` \| `uninstall` | Start the daemon at login, or stop doing so |
| | `truss desktop` | Open TRUSS Desktop |
| **Pipelines** | `truss register [<path>] [--name <name>]` | Make a file known to the daemon |
| | `truss unregister <name>` | Forget it; the file and history stay |
| | `truss pipelines` | List registrations, next and last run |
| | `truss pause <name>` \| `resume <name>` | Stop or restart its schedules |
| **Runs** | `truss trigger <name> [--follow]` | Start a run now |
| | `truss runs [<name>]` | List runs, newest first |
| | `truss logs <run-id> [--follow]` | Show a run log |
| | `truss cancel <run-id>` | End a queued or running run |

Details and exit codes: [command line reference](docs/cli.md).

## How it fits together

```mermaid
flowchart LR
    yaml[/"truss.yaml"/]
    cli["truss CLI"]
    desktop["TRUSS Desktop"]
    subgraph daemon["truss daemon"]
        registry["Registry"]
        scheduler["Scheduler"]
        runs["Run queue and history"]
    end
    engine["Engine"]
    shell["Host shell"]

    cli -- "truss run" --> engine
    cli -- "HTTP, loopback" --> daemon
    desktop -- "HTTP, loopback" --> daemon
    yaml -.-> engine
    yaml -. registered .-> registry
    scheduler --> runs --> engine --> shell
```

`truss run` and the daemon drive the same engine, so a pipeline behaves the same either way.
The engine knows nothing about the daemon, and the daemon knows nothing about its clients.
The full map is in [docs/architecture.md](docs/architecture.md).

## Documentation

| Read this | When you want to |
|---|---|
| [Pipeline file reference](docs/pipeline-reference.md) | Know every field, cron syntax, and recipes |
| [Command line reference](docs/cli.md) | Look up a command or an exit code |
| [The daemon](docs/daemon.md) | Understand registration, schedules, missed runs, concurrency, the HTTP API |
| [TRUSS Desktop](docs/desktop.md) | See what the UI offers |
| [Architecture](docs/architecture.md) | Find your way around the code |
| [Glossary](docs/GLOSSARY.md) | Check what a word means here |
| [Milestone](docs/MILESTONE.md) | See what v0.1.0 includes and what waits |
| [Decisions](docs/decisions/) | Learn why something is the way it is |
| [Specifications](specs/) | Read the requirements behind each feature |

## Roadmap

After v0.1.0, roughly in this order. Scope and order may change.

- **Desktop:** a packaged window with tray icon, notifications and start at login
- **Pipelines:** matrix builds, artifacts
- **Triggers:** file changes and webhooks
- **Execution:** containers, then remote workers
- **Ecosystem:** SDKs and CI-provider bridges, so the same file runs in CI

## Development

```bash
gofmt -l .                                   # must print nothing
go vet ./...
go test -race ./...                          # unit, integration and CLI tests
golangci-lint run ./...
go build -o bin/truss ./cmd/truss
bash .github/scripts/e2e-test.sh bin/truss   # the built binary, including a real daemon
```

- [`docs/architecture.md`](docs/architecture.md) maps the packages and their boundaries.
- [`.specify/memory/constitution.md`](.specify/memory/constitution.md) holds the engineering principles: behavior is specified, tested, and safe to execute.
- [`specs/`](specs/) holds one folder per feature; `spec.md` travels in the same pull request as the code.
- [`AGENTS.md`](AGENTS.md), [`CLAUDE.md`](CLAUDE.md) and [`docs/agents.md`](docs/agents.md) describe how coding agents work here.

TRUSS Desktop needs no build step: edit the files in `internal/webui/assets/`, rebuild the binary, reload the page.

### Releasing

Releases are cut by pushing a semantic version tag such as `v0.1.0` or `v0.2.0-rc.1`. The release workflow refuses to publish unless:

1. The `// version: X.Y.Z` line in `go.mod` equals the tag.
2. The module path follows Go's major-version rule: no suffix for v0 and v1, `/vN` from v2 onward.
3. `const Version` in `cmd/truss/version.go` (what `truss version` prints) equals the tag.
4. `internal/webui/VERSION`, the version of TRUSS Desktop, equals the tag. It is a file of its own, apart from the Go version.
5. `CHANGELOG.md` has `## [X.Y.Z] - YYYY-MM-DD` as its latest section, with entries, the tag's date, and only the headings Added, Changed, Deprecated, Removed, Fixed and Security.

It then runs the full CI suite, builds Linux, macOS and Windows binaries (amd64 and arm64), and publishes a GitHub Release with checksums. The release notes are the changelog section followed by the notes GitHub generates (contributors and history). Pre-release tags are published as pre-releases. Prepare a release on a `release/<version>` branch.

### Branches and commits

Branches are named `<type>/<name>` with type `bugfix`, `feature`, `hotfix`, `release`, `task` or `chore`. Every commit is authored by `afri-bit`; a `Co-authored-by` trailer is allowed, and Claude may be named as co-author. The `Repository rules` workflow checks both on every push and pull request (see [ADR 0014](docs/decisions/0014-branch-prefixes-commit-identity-and-release-gate.md)).

## License

TRUSS is released under the [Apache License 2.0](LICENSE). See [NOTICE](NOTICE).
