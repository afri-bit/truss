# AGENTS.md

Single source of agent instructions for TRUSS. Applies to every coding agent
(Claude Code, Cursor, Codex, Copilot, …) and to humans.

## Project

TRUSS is a provider-independent, portable CI pipeline runner written in Go.
Pipelines are YAML files that run identically on a developer machine, Jenkins,
or GitHub Actions, on Linux, macOS, and Windows. Each step declares whether it
runs on the host or in a container.

Components:

- **`truss` CLI** (cobra): `run`, `daemon`, `bind`, `status`, `logs`, `init`.
- **`trussd` daemon** (same binary, `truss daemon`): scheduler (robfig/cron,
  one lock per binding), binding registry (pipeline → workspace + triggers),
  HTTP + SSE API over a Unix socket.
- **Core** (no daemon imports): pipeline parser/validator (schema
  `version: 1`), engine (sequential step lifecycle), `Executor` interface with
  `HostExecutor` (os/exec) and `ContainerExecutor` (Docker Go SDK), `EventSink`
  interface.
- **Persistence**: `Store` interface, `SQLiteStore` (modernc.org/sqlite, pure
  Go), `MemStore` for direct mode.
- **Run modes**: direct (in-process, logs to stdout) and daemon (persisted
  history, streamed events).

SDKs and integrations may use other languages and must not depend on
Go-specific implementation details.

### Scope

- **v0.1.0 in scope**: sequential steps, host and container executors, daemon
  with cron and manual triggers, direct mode, SQLite run history, explicit
  workspaces via bindings.
- **Out of scope for v0.1.0**: DAG or parallel steps, secrets backend,
  caching, artifacts, provider adapters (GitHub/Jenkins import), web UI
  (planned for v0.3; `web/` is reserved).

Do not implement out-of-scope features. Do not add speculative hooks for them.

## Sources of Truth

| Artifact | Purpose |
|----------|---------|
| `.specify/memory/constitution.md` | Project-wide engineering principles and constraints. **Prevails** over this file. |
| `specs/<NNN-feature>/spec.md` | Required behavior and intent (EARS acceptance criteria). |
| `specs/<NNN-feature>/plan.md` | Agreed technical design. |
| `specs/<NNN-feature>/tasks.md` | Ordered implementation tasks. |
| `docs/adr/` | Architecture decisions and their rationale. |
| `docs/architecture.md` | Components, package layout, import rules. |
| Source code | The current implementation, not automatically the intended behavior. |

If the implementation and a spec disagree, do not silently change either side.
Report the discrepancy first. If this file and the constitution disagree, the
constitution prevails; surface the conflict.

## Architecture Rules

These rules are normative in the constitution under "Architecture
Constraints". They are enforced by `make lint` (depguard, forbidigo).

1. **Dependencies point inward.** `pkg/engine` imports nothing from the
   daemon, the API, or store packages. `pkg/` never imports `internal/` or
   `cmd/`.
2. **Typed events, not strings.** The engine emits typed events to an
   `EventSink`. Renderers and sinks format output. Library code never prints.
3. **Three versioned public contracts**: the pipeline schema, the CLI
   surface, and the HTTP API (`/v1`). Changing any of them requires an
   approved spec **and** an ADR.
4. **Cross-platform is a feature.** Make no shell assumptions. Use
   `path/filepath`. Handle Windows paths and CRLF line endings.

## Repository Layout

```text
cmd/truss/          CLI + daemon entrypoint; the only place that wires, prints, exits
pkg/pipeline/       schema, parser, validator (innermost; imports no TRUSS pkgs)
pkg/engine/         sequential step lifecycle, typed events, EventSink
pkg/executor/       host and container executors
pkg/store/          Store contract; SQLite and in-memory implementations in sub-packages
pkg/api/            HTTP /v1 types and client
internal/daemon/    scheduler, binding registry, HTTP + SSE server
internal/socket/    Unix socket path resolution and permissions
internal/sinks/     EventSink implementations (terminal, store recorder, SSE)
web/                reserved for the v0.3 web UI (no code yet)
docs/               architecture, ADRs, development and user docs
specs/              feature specifications (+ README index, _template/)
examples/           illustrative pipelines
```

Do not add top-level directories or packages without justifying them in a
plan. Do not assume structure from general conventions; read what is here.

## Commands

```sh
make build             # bin/truss with version ldflags
make test              # go test ./...
make test-race         # go test -race ./...  (needs cgo)
make test-integration  # adds -tags integration (needs Docker)
make lint              # golangci-lint + spec-check
make fmt               # gofmt + goimports
make tidy              # go mod tidy
make cover             # coverage summary
```

Run `make fmt lint test` before every commit.

## Development Workflow (spec-driven)

Full procedure: `docs/development/spec-driven-workflow.md`.

1. **Specify**: create `specs/NNN-short-name/spec.md` from the template and
   add it to `specs/README.md`.
2. **Clarify**: resolve every `[NEEDS CLARIFICATION]`. Never pick an answer
   arbitrarily when it changes behavior or architecture.
3. **Plan**: write `plan.md`. Write an ADR for architecturally significant
   decisions.
4. **Tasks**: write `tasks.md`. Each task is small enough for one PR and
   references the FR IDs it satisfies.
5. **Implement**: one task per PR, tests first.
6. **Verify**: confirm the implementation converges with the spec.

Each spec file's front matter carries
`status: draft | approved | implementing | done`.

- **No implementation PR without an approved spec.** If the spec is `draft`,
  stop and say so.
- An implementation PR links the spec and lists the task IDs it completes.
- Mechanical changes (formatting, typos, dependency bumps, equivalent
  refactors) need no spec unless they alter observable behavior.

### Before implementing

1. Read this file and `.specify/memory/constitution.md`.
2. Find the feature under `specs/`. Read `spec.md`, `plan.md`, and
   `tasks.md`, and check the status.
3. Read the ADRs the plan references.
4. Inspect the existing code you will touch.

### Specification discipline

- `spec.md` says what and why. Implementation details appear only when
  externally observable. Technical decisions go in `plan.md`; work goes in
  `tasks.md`.
- Do not invent requirements or silently expand scope.
- Surface material ambiguity instead of choosing.

## Coding Conventions

Details and examples: the `go-conventions` skill (`.claude/skills/`) and
`.cursor/rules/`.

- Follow Effective Go and the Google Go Style Guide. Code is `gofmt`ed and
  `goimports`ed (local prefix `github.com/afri-bit/truss`).
- Wrap errors with context: `fmt.Errorf("load pipeline %q: %w", path, err)`.
  Compare with `errors.Is` and `errors.As`. Never panic on user input.
- Pass `context.Context` as the first parameter of anything that blocks,
  does I/O, or runs processes. Never store it in a struct.
- Define interfaces at the consumer, keep them small, and return concrete
  types.
- No global mutable state and no `init()` side effects. Inject dependencies
  through constructors.
- Log with `log/slog` and an injected `*slog.Logger`. Use static messages
  and snake_case keys. Logs are diagnostics, not product output.
- Document every exported identifier with a sentence that starts with its
  name.
- Paths: use `filepath`. Commands: use `exec.CommandContext` with an
  explicit argv. Never build shell strings.
- Never log or echo secrets, `env` values, or `run` content in errors.
- Add a dependency only when it solves a concrete problem, and justify it
  in the plan.

## Testing Policy

- Every behavior change ships with tests that would fail without it.
  Regression fixes include a regression test.
- Prefer table-driven tests with `t.Run`. Use `t.TempDir()`, `t.Setenv()`,
  and `t.Cleanup()`. Never touch the real home directory.
- Name the FR IDs covered in test names or comments, for example
  `// Covers FR-016.`
- Put fixtures in `testdata/`. Use golden files for error output and
  rendered events.
- Docker-dependent tests start with `//go:build integration`. Unit tests
  must pass without Docker or network access.
- Tests must pass on Linux, macOS, and Windows. Skip a platform only with
  `t.Skip` and a reason.
- `go test -race` must pass. Do not weaken or delete tests to make code
  pass.

## Commits and Pull Requests

- Use [Conventional Commits](https://www.conventionalcommits.org/):
  `type(scope): summary`. Types: `feat`, `fix`, `docs`, `test`,
  `refactor`, `perf`, `build`, `ci`, `chore`. Breaking changes use `!` and
  a `BREAKING CHANGE:` footer.
- Use the imperative mood, at most 72 characters, and no trailing period.
  The body explains why.
- One logical change per commit. One task per implementation PR.
- PRs follow `.github/pull_request_template.md`: link the spec, list task
  IDs, and complete the checklist.
- Update `CHANGELOG.md` (Unreleased) for user-visible changes.
- Do not commit generated binaries, coverage files, or local settings.

## Generated and Tooling Files

- `.specify/`: Spec Kit infrastructure and SDD templates. Edit only for
  workflow changes. Keep `specs/_template/` identical (`make spec-check`).
- `.cursor/`: Cursor integration. `.claude/`: Claude Code skills and
  subagents.
- Do not move project requirements into tool directories. The product must
  not depend on any agent tooling at runtime.

## Completion Checklist

Before declaring work complete:

1. The behavior matches the active spec, and every touched FR has a test.
2. The implementation follows `plan.md`, or deviations are documented.
3. Task checkboxes are ticked and spec statuses updated, along with
   `specs/README.md`.
4. `make fmt lint test` passes, and `make test-integration` passes when
   executors are touched.
5. There are no unintended changes outside the task's scope.
6. Unresolved limitations or deviations are reported clearly.

Never claim completion while tests fail or specified behavior is missing.
