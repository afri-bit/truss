# Feature Specification: Daemon configuration

**Spec**: `027-daemon-config` | **Created**: 2026-11-25 | **Status**: Implemented

Extends [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md).
User docs: [`docs/daemon.md`](../../docs/daemon.md#configuration). Decision:
[ADR 0025](../../docs/decisions/0025-daemon-settings-live-in-a-file-in-the-truss-home.md). Risk class: tier 2 (a new file
the daemon reads, CLI output). Scope set by the maintainer on 2026-11-25 (milestone v0.3.0): "the
configuration of the daemon is not really transparent in the documentation".

## Why

The daemon's settings were two flags of `truss daemon start`, given again at every start, and the
history limit could not be changed at all. Nothing showed what the daemon would use.

## User stories

1. **Set it once** (P1). I write `max_runs: 2` in `~/.truss/config.yaml` and every start uses it.
2. **See what is in effect** (P1). `truss daemon config` shows each setting, its value and where it
   comes from.
3. **Override for one start** (P2). A flag or a `TRUSS_DAEMON_*` variable wins over the file.

## Requirements

- **FR-001** The daemon reads `config.yaml` in the TRUSS home when it starts. A missing or empty file
  is fine. Settings: `address` (default `127.0.0.1:7878`), `max_runs` (default 4, at least 1) and
  `history` (default 200, at least 1: finished runs kept with their logs).
- **FR-002** Each setting resolves from, first match wins: the flag of `truss daemon start` or
  `run` (`--address`, `--max-runs`, `--history`), the environment variable (`TRUSS_DAEMON_ADDRESS`,
  `TRUSS_DAEMON_MAX_RUNS`, `TRUSS_DAEMON_HISTORY`; an empty value counts as unset), the file, the
  default.
- **FR-003** An unknown or repeated key, a value that is not a single value, invalid YAML, an
  invalid value or a non-loopback address stops the daemon from starting; `truss daemon start`
  checks before it launches the service, so the error is on the screen. The error with an error that names the setting and where the value came
  from, never the value. An empty flag counts as not given. The file is at most 64 KiB and must be a regular file (no link).
- **FR-004** `truss daemon config [--json]` prints the file path and whether it exists, then each
  setting with its value and source (`default`, `config.yaml` or the variable's name), and how to
  set each one. It needs no daemon and exits 1 when the file is invalid. `--json` prints
  `{file, exists, settings: [{key, value, source, default, env, flag}]}`.
- **FR-004a** `--history` goes with `daemon start` and `run` only; `daemon install` refuses it,
  because the login item does not carry it: the file does.
- **FR-005** The address keeps the rule of spec 006: loopback only.

## Compatibility

Additive. `--address` and `--max-runs` keep their meaning. The default history stays 200.
`truss daemon install` keeps passing only the flags it was given; the login-started daemon reads
the file at each start.

## Acceptance

`internal/config/config_test.go` (defaults, precedence, errors) and `cmd/truss/config_test.go`
(`daemon config` text and JSON, and a `daemon run` that uses the file's `max_runs`).
