# Development Setup

## Prerequisites

| Tool | Version | Needed for |
|------|---------|------------|
| Go | as in [`go.mod`](../../go.mod) | everything |
| make + POSIX shell | any | Makefile targets (on Windows, use Git Bash or WSL) |
| Git | 2.30+ | checkout with LF normalization |
| C compiler (gcc or clang) | any | `make test-race` (the race detector needs cgo) |
| Docker | 24+ | `make test-integration` and the container executor |

If your installed Go is older than the version in `go.mod`, the default
`GOTOOLCHAIN=auto` downloads the right toolchain automatically.

## First Build

```sh
git clone https://github.com/afri-bit/truss.git
cd truss
make tools          # installs the pinned golangci-lint into ./bin
make build          # produces ./bin/truss (truss.exe on Windows)
./bin/truss         # prints version, commit, build date, Go version, platform
make test lint
```

`make help` lists every target:

| Target | What it does |
|--------|--------------|
| `build` | Builds `bin/truss` with version metadata from `git describe` |
| `test` | `go test ./...` |
| `test-race` | `go test -race ./...` |
| `test-integration` | `go test -race -tags integration ./...` (Docker required) |
| `lint` | golangci-lint (including architecture rules) and `spec-check` |
| `fmt` | gofmt and goimports via golangci-lint |
| `tidy` | `go mod tidy` |
| `cover` | Coverage profile and total |
| `spec-check` | Fails if `specs/_template/` differs from `.specify/templates/` |
| `tools` | Installs pinned tools into `bin/` |
| `clean` | Removes build and coverage output |

Override build metadata when needed, for example
`make build VERSION=v0.1.0-rc.1`.

## Line Endings

The repository stores text with LF line endings (`.gitattributes`). On
Windows, keep `core.autocrlf=false` (or `input`) so test fixtures stay
byte-identical across platforms. `.editorconfig` configures editors to
match.

## Editors

- **VS Code**: install the Go extension and set
  `"go.lintTool": "golangci-lint"` and
  `"go.lintFlags": ["--path-mode=abs", "--fast-only"]`. Point it at
  `./bin/golangci-lint` or a matching global install.
- **GoLand**: enable gofmt and goimports on save, and add golangci-lint as
  a file watcher or use the plugin.
- Build tags: add `integration` to your editor's build tags
  (`"go.buildTags": "integration"`) so integration tests type-check.

## Coding Agents

Agents start from [AGENTS.md](../../AGENTS.md).

- **Claude Code** loads `CLAUDE.md`, which imports `AGENTS.md`, plus the
  skills and subagents in `.claude/`.
- **Cursor** uses `.cursor/rules/` and the Spec Kit commands in
  `.cursor/skills/` (`/speckit-specify`, `/speckit-plan`, `/speckit-tasks`,
  …).

Spec Kit (`.specify/`) is development tooling only. Nothing in the product
depends on it at runtime.
