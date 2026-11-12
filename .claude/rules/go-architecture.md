---
paths:
  - "**/*.go"
  - "specs/**/plan.md"
---

# Go packages and boundaries

TRUSS is one Go module. The engine is independent of the CLI (constitution principle IV).

| Package | Job | Must not |
|---|---|---|
| `cmd/truss` | Flags, I/O, exit codes, user-facing output; starts the daemon process | Hold execution or scheduling logic |
| `internal/pipeline` | Load, validate and plan a pipeline definition | Run anything or print |
| `internal/cron` | Parse cron expressions, compute the next match | Know what a match starts |
| `internal/engine` | Run a validated plan through the executor, return a result | Print, read flags, know the daemon |
| `internal/executor` | Run one step command on the host shell, report exit code and output | Decide ordering |
| `internal/runlog` | Format and write the run log | Execute steps |
| `internal/home` | Locate the TRUSS home, read and write its JSON files | Hold state |
| `internal/secrets` | Keep named secret values in a private file; remove values from output | Know pipelines or the engine |
| `internal/registry` | Remember registered pipeline files | Read pipeline content |
| `internal/scheduler` | Decide which schedules fire or were missed at a given time | Own a timer or start runs |
| `internal/runs` | Queue, execute and remember runs | Know HTTP or schedules |
| `internal/loginstart` | Generate and install the per-user login item (systemd, launchd, Task Scheduler) behind runner and file interfaces | Know the daemon or its API, or print |
| `internal/api` | Daemon wire types and Go client | Contain server logic |
| `internal/daemon` | Wire the service together, serve the HTTP API | Reimplement the packages it wires |
| `internal/webui` | TRUSS Desktop's embedded static files | Contain Go logic |

Dependencies point one way: surfaces (`cmd`, `webui`) to service (`daemon`, `runs`, ...) to core
(`engine`, `pipeline`, ...). The map is `docs/architecture.md`.

Update this table in the same change when a package is added or its job changes.

## Rules

- One package, one purpose. No `internal/util`, no `pkg/` public API unless a spec requires it.
- `internal/*` returns results and errors. Only `cmd/` writes user-facing output.
- Interfaces live where they are consumed, are small, and exist only at a real boundary
  (several implementations, isolation, or testing). Do not add one "for later".
- Extend behavior with a new type that satisfies a small interface. Honor cancellation, wrap
  errors, never panic on user input.
- Do not add layers, frameworks or top-level directories for hypothetical needs.
- Keep external contracts (pipeline file, CLI output, run log) free of Go-specific details so
  other languages can consume them.

```go
// Bad: the engine prints.
func (e *Engine) Run(ctx context.Context) error { fmt.Println("starting"); return e.exec.Start(ctx) }

// Good: the engine returns a result; cmd/ prints.
func (e *Engine) Run(ctx context.Context) (Result, error) { return e.exec.Start(ctx) }
```
