---
name: go-conventions
description: TRUSS Go coding conventions with examples - error wrapping, context propagation, interface placement, table-driven tests, no global state, slog logging, cross-platform paths and processes. Use whenever writing, editing, or reviewing Go code in this repository.
---

# Go conventions

The constitution (Principle II) is binding; this is the practical guide.

## Errors

- Wrap with context using `%w`; describe the operation, not "failed to".
- Inspect with `errors.Is` / `errors.As`, never by string matching.
- Sentinel errors: `var ErrNotFound = errors.New("store: run not found")`.
  Typed errors when callers need fields; name them `XxxError`.
- Return errors; never `panic` on user input or I/O failure. Handle each
  error once: wrap and return, *or* log and continue — not both.

```go
f, err := os.Open(path)
if err != nil {
	return nil, fmt.Errorf("open pipeline %q: %w", path, err)
}
```

## Context

- `ctx context.Context` is the first parameter of anything that blocks,
  performs I/O, or runs processes. Never store it in a struct; never pass
  `nil` (use `context.TODO()` only in tests or transitional code).
- Honor cancellation: `exec.CommandContext`, `select` on `ctx.Done()`, and
  propagate `ctx.Err()` wrapped.

## Interfaces

- Define interfaces in the **consuming** package, as small as the consumer
  needs; return concrete types from constructors.
- Exceptions are documented contracts with several implementations used by
  several consumers (`store.Store`, `engine.EventSink`, `engine.Executor`).
- Assert implementation at compile time: `var _ engine.Executor = (*Host)(nil)`.

```go
// in pkg/engine (consumer). Only pipeline and standard-library types appear
// in the signature, so pkg/executor can implement it without importing the
// engine (which depguard forbids).
type Executor interface {
	Execute(ctx context.Context, step pipeline.Step, stdout, stderr io.Writer) (exitCode int, err error)
}
```

## No global state

- No package-level mutable variables, no `init()` with side effects, no
  singletons. Inject dependencies through constructors (`NewDaemon(cfg,
  store, logger)`). `gochecknoglobals` enforces this outside `cmd/`.
- Configuration flows from `cmd/` downward as explicit structs.

## Logging

- `log/slog` only, via an injected `*slog.Logger`; never the global logger
  in library code (`sloglint` enforces).
- Static messages, snake_case keys, attributes for data:
  `logger.InfoContext(ctx, "step finished", "step_id", id, "exit_code", code)`.
- Logs are diagnostics. User-facing progress is **typed events** to an
  `EventSink`; library code never prints (`forbidigo` enforces).
- Never log secrets, `env` values, or `run` content.

## Tests

- Table-driven with named cases and `t.Run`; compare with clear
  `got/want` messages.
- `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`; no shared mutable fixtures;
  `t.Parallel()` where safe.
- Fixtures in `testdata/`; golden files for error lists and rendered
  output, updated via a `-update` flag.
- Docker tests: first line `//go:build integration`.
- Reference covered FRs: `// Covers FR-016.`

```go
func TestValidateStepID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "simple", id: "build"},
		{name: "uppercase rejected", id: "Build", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStepID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateStepID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}
```

## Cross-platform

- `path/filepath` for OS paths; `path` only for slash-separated logical
  paths (e.g. `workdir` in YAML, URL paths).
- Never build shell strings; `exec.CommandContext(ctx, argv[0], argv[1:]...)`.
- Treat CRLF and a UTF-8 BOM as possible input; compare text with
  normalized line endings in tests.
- Guard OS-specific code with `_windows.go` / `_unix.go` files, not runtime
  `if` chains, when behavior differs materially.

## Packages and API

- One purpose per package; no `util`/`common`. No stutter
  (`engine.Run`, not `engine.EngineRun`).
- Keep exported API minimal; everything under `pkg/` is importable by
  others. Respect the import rules in `docs/architecture.md`.
