# 0003. A local daemon with an HTTP API and plain-file state

- Status: Accepted
- Date: 2026-11-12

## Context

Schedules, a run history and a UI need a process that outlives one `truss run`. The maintainer
asked for a daemon "like Docker has", pipelines that can be registered and removed, cron
schedules that respect a laptop's life, and concurrency.

## Decision

1. **One binary.** The daemon is `truss daemon run`; `truss daemon start` launches it in the
   background. There is no second executable to install or keep in step.
2. **HTTP and JSON on loopback TCP**, default `127.0.0.1:7878`, under `/api/v1`. A Unix socket
   would be tidier on Linux and macOS but needs a second transport on Windows, and a browser
   UI cannot use it.
3. **Access control.** The daemon refuses non-loopback addresses, requires a random bearer
   token stored in a user-only file in the TRUSS home, and rejects requests whose `Host` header
   is not this machine. The token rotates at every start.
4. **Registration stores a path, not a copy.** The file stays the source of truth and is read
   again before each run and when it changes on disk.
5. **State is JSON and text files** under `~/.truss` (`TRUSS_HOME` overrides), replaced
   atomically. No database, no new module.
6. **Missed schedules are skipped by default** and recorded. `missed: run` asks for one
   catch-up run. "Missed" means more than two minutes late, measured against the last time the
   scheduler saw the clock, which is saved to disk.
7. **Three concurrency controls**: runs at once (daemon), overlap per pipeline (`queue`, `skip`,
   `parallel`), and steps at once (`pipeline.concurrency`).
8. **`truss run` stays independent** of the daemon and ignores schedules. It is not handed to
   the daemon even when one runs, because the run would then use the daemon's environment
   instead of the caller's. Tracked runs go through `register` and `trigger`.
   *Decided with the maintainer, 2026-11-12.* `truss run --track` later added opt-in reporting
   to the daemon without handing the run over ([ADR 0006](0006-tracked-local-runs.md)).
9. **The daemon is the server, started separately.** Clients (CLI, Desktop) never start or stop
   it, and Desktop is unusable without it. *Decided with the maintainer, 2026-11-12.*
10. **Clients poll.** Run logs are fetched by byte offset. Server-sent events can be added later
   without changing existing endpoints.

## Consequences

- The CLI, TRUSS Desktop and any script share one contract; other languages need only HTTP.
- A fixed default port can collide with another program; `--address` and port `0` cover that.
- Any process running as the user can read the token. That equals the trust already placed in
  the user's shell and is documented.
- Polling costs a request per second per open view. Fine for one machine.
- The daemon does not start at login by itself yet; the docs show how to arrange it.

## Alternatives considered

- **Separate `trussd` binary**: clearer name, but two artifacts to ship for no gain today.
- **gRPC**: typed, but a new dependency and no direct browser support.
- **SQLite for state**: better queries, but a cgo or third-party dependency for a history of a
  few hundred small records.
- **Catch up on every missed time**: surprising and potentially destructive after a week off.
