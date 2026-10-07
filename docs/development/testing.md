# Testing

TRUSS executes other people's commands on three operating systems, so tests
are the main defense against regressions. The binding rules are in the
[constitution](../../.specify/memory/constitution.md) (Principle III); this
page explains the practice.

## Layers

| Layer | Location | Runs in | Command |
|-------|----------|---------|---------|
| Unit | `*_test.go` beside the code | every CI job, every OS | `make test` |
| Race | the same tests with `-race` | every CI job, every OS | `make test-race` |
| Integration | `*_test.go` files tagged `integration` | Ubuntu CI job with Docker | `make test-integration` |
| Contract | golden files for schema errors, CLI output, and `/v1` payloads | with unit tests | `make test` |

Unit tests must not require Docker, network access, or a running daemon.
They may run real host processes (for example, the test binary itself as a
helper process) when testing the host executor.

## Integration Tests

Tests that need Docker (container executor, end-to-end daemon runs) start
with a build constraint:

```go
//go:build integration

package executor_test
```

They are excluded from `go test ./...` and included by
`go test -tags integration ./...`. Integration tests:

- pull small, pinned images (by tag and, where possible, digest);
- clean up the containers, networks, and volumes they create
  (`t.Cleanup`);
- use `t.Skip` with a reason only when Docker is unavailable *locally*.
  CI always has Docker, so a skip there is a failure to investigate.

## Conventions

- **Table-driven** tests with named cases and `t.Run`.
- **Traceability**: reference the acceptance criteria a test covers, for
  example `// Covers FR-016, FR-030.`
- **Fixtures** live in `testdata/`, which the Go tool ignores for builds.
- **Golden files** (`testdata/*.golden`) hold expected error lists and
  rendered output. Regenerate them with `go test ./pkg/... -update`, and
  review the diff like code.
- **Isolation**: use `t.TempDir()`, `t.Setenv()`, and `t.Cleanup()`. Never
  write to the repository, the home directory, or a fixed socket path.
- **Determinism**: inject clocks and ID generators. Do not use `time.Sleep`
  for synchronization; wait on channels or events instead.
- **Typed events**: assert on the slice of events an in-memory sink
  received, not on rendered text, unless you are testing a renderer.

## Cross-Platform

- CI runs lint and race tests on Ubuntu, macOS, and Windows for every pull
  request into `main`.
- Compare paths with `filepath`. Build expected paths with
  `filepath.Join`, never string literals containing `/`.
- Parsers get fixture variants with CRLF line endings and a UTF-8 BOM.
- Platform-specific tests live in `_windows_test.go` or `_unix_test.go`
  files, or skip explicitly with `t.Skip("reason")` and an issue link.

## Coverage

`make cover` prints the total and writes `coverage.out`. There is no hard
threshold. Reviewers expect every FR a change implements to be covered,
including error paths.
