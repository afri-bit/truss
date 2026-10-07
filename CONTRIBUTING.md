# Contributing to TRUSS

Thanks for your interest in TRUSS. This guide explains how changes get from
an idea to `main`. By participating you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

## How TRUSS Is Developed

TRUSS is **spec-driven**. Behavior is written down and agreed before it is
implemented:

```text
issue → spec.md (draft → approved) → plan.md → tasks.md → one PR per task → done
```

- **No implementation PR without an approved spec.** Changes to observable
  behavior start with a spec under [`specs/`](specs/README.md).
- Changes to a public contract (pipeline schema, CLI surface, HTTP API `/v1`)
  also need an [ADR](docs/adr/README.md).
- Bug fixes that restore specified behavior reference the spec and its FR ID.
- Mechanical changes (typos, formatting, dependency bumps, behavior-preserving
  refactors) need no spec.

The full procedure, status lifecycle, and EARS cheat sheet are in
[docs/development/spec-driven-workflow.md](docs/development/spec-driven-workflow.md).
Project-wide principles live in the
[constitution](.specify/memory/constitution.md).

### Proposing a feature

1. Open a **feature request** issue that describes the problem.
2. If it fits the roadmap, write the spec (or ask a maintainer to). Spec PRs
   are titled `docs(spec): NNN short name`.
3. Once the spec is `approved`, the plan and tasks follow, then
   implementation.

### Reporting a bug

Open a **bug report** with a minimal pipeline that reproduces it. Report
security issues privately (see [SECURITY.md](SECURITY.md)).

## Development Setup

Requirements:

- Go (the version in [`go.mod`](go.mod); `GOTOOLCHAIN=auto` fetches it)
- `make` and a POSIX shell (on Windows, use Git Bash or WSL)
- Docker, only for integration tests
- A C compiler, only for `make test-race` (cgo)

```sh
git clone https://github.com/afri-bit/truss.git
cd truss
make tools   # installs the pinned golangci-lint into ./bin
make build test lint
```

More detail: [docs/development/setup.md](docs/development/setup.md) and
[docs/development/testing.md](docs/development/testing.md).

## Making a Change

1. Fork the repository and branch from `main`, for example
   `feat/001-duplicate-step-ids`.
2. Implement **one task** from an approved `tasks.md`. Write the tests
   first.
3. Run `make fmt lint test` (and `make test-integration` if you touched
   executors).
4. Tick the task in `tasks.md`, update the spec status if this is the first
   or last task, and add a `CHANGELOG.md` entry for user-visible changes.
5. Open a PR using the template. Link the spec and list the task IDs.

Architecture rules are enforced by the linter: the engine must not import
the daemon, API, or store packages; library code emits typed events and
never prints; and code must work on Linux, macOS, and Windows. See
[AGENTS.md](AGENTS.md) for the complete conventions. They apply to humans
and AI agents alike.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```text
<type>(<scope>): <imperative summary, max 72 chars>

<why the change is needed>

Refs: specs/001-pipeline-schema-v1 T007
```

- **Types**: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`,
  `ci`, `chore`, `revert`.
- **Scopes** usually name a package or area: `pipeline`, `engine`,
  `executor`, `store`, `api`, `daemon`, `cli`, `spec`, `adr`.
- **Breaking changes**: `feat(api)!: …` plus a `BREAKING CHANGE:` footer.

Keep commits focused, one logical change each. Squash fixups before review.

## Review

- At least one maintainer approval is required, and CODEOWNERS applies.
- CI must be green: lint and race tests on Linux, macOS, and Windows, plus
  integration tests with Docker.
- Reviewers check the change against the spec. Behavior beyond the spec is
  rejected, even if it is useful. Propose a spec change instead.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
