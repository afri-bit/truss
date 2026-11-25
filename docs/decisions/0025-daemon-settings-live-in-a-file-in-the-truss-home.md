# 0025. Daemon settings live in a file in the TRUSS home

- Status: Proposed
- Date: 2026-11-25

## Context

The daemon's settings were flags of `truss daemon start`, repeated at every start, and the run
history limit was fixed. The maintainer found the daemon's configuration not transparent
(milestone v0.3.0, 2026-11-25). A new file and new variables are public contracts
(constitution V).

## Decision

1. `config.yaml` in the TRUSS home, YAML like the flow and project files, read at every start.
2. A flag of `daemon start`/`run` wins over a `TRUSS_DAEMON_*` variable, which wins over the file,
   which wins over the default: the nearer to the command, the stronger.
3. An unknown or repeated key, or a bad value, stops the daemon from starting, with an error that
   names the setting and where its value came from. `daemon start` checks before it launches the
   service, so the error is on the screen.
4. The file must be a regular file, at most 64 KiB; a link is refused, like the other files of the
   home.
5. `truss daemon config [--json]` shows every value and its source. Its JSON is
   `{file, exists, settings: [{key, value, source, default, env, flag}]}`.
6. A new package, `internal/config`, owns reading and resolving; it knows nothing of the daemon.

## Consequences

- One place to look; the login item no longer needs to carry settings.
- `--history` is not stored in a login item; it belongs in the file.
