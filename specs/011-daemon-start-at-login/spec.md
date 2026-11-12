# Feature Specification: Daemon start at login

**Spec**: `011-daemon-start-at-login` | **Created**: 2026-11-12 | **Status**: Implemented (generation and fake-runner tests only; see Verification)

**Input**: "The daemon does not start at login, so after a reboot nothing runs until the user
starts it." The rule from [006](../006-daemon-registry-and-scheduling/spec.md) stands: the daemon
is the server, started separately. Desktop and the CLI never start it implicitly.

User-facing documentation: [`docs/daemon.md`](../../docs/daemon.md#starting-at-login) and
[`docs/cli.md`](../../docs/cli.md). Risk tier 2 (process execution at login, per-user system
state). Written alongside the implementation (constitution 2.0.0, principle I).

## User stories

1. **Survive a reboot** (P1). I run one command once and my schedules work after every login.
   *Independent test*: `truss daemon install`, log out and in, `truss daemon status` says running.
2. **Take it back** (P1). I run one command and TRUSS leaves nothing behind.
   *Independent test*: `truss daemon uninstall`, then check the unit, agent or task is gone.
3. **See the state** (P2). `truss daemon status` tells me whether login start is installed.

## Requirements

### Commands

- **FR-001** `truss daemon install [--address <host:port>] [--max-runs <n>]` makes the daemon
  start at login by running `<this binary> daemon run` with the same flags. `--address` must be
  a loopback address, as for `daemon run`; it is checked at install, not at the next login.
- **FR-002** `truss daemon uninstall` removes what install created and stops the daemon that
  login start launched. It takes no flags. When nothing is installed it says so and exits 0.
- **FR-003** `truss daemon status` adds one line, `at login`, saying `installed` or
  `not installed`, in both the running and the not-running output. Exit codes do not change.
  Where the state cannot be read the line says `unknown`; status never fails because of it.
- **FR-004** Installing again replaces the earlier installation (new binary path or flags) and
  says `updated`.
- **FR-005** Nothing runs with root or administrator rights, and nothing is written outside the
  user's own directories or per-user scheduler.

### Per platform

- **FR-006 Linux.** A systemd user unit `~/.config/systemd/user/truss.service` with
  `ExecStart=<binary> daemon run [flags]`, `Restart=on-failure`, `WantedBy=default.target`,
  enabled with `systemctl --user`. The output mentions `loginctl enable-linger` for running
  without a login session.
- **FR-007 macOS.** A launchd agent `~/Library/LaunchAgents/io.github.afri-bit.truss.plist` with
  `RunAtLoad` and `KeepAlive` on failure only (`SuccessfulExit` false), output going to
  `daemon.log` in the TRUSS home, loaded into the `gui/<uid>` domain with `launchctl`.
- **FR-008 Windows.** A per-user Task Scheduler task `truss-daemon` that runs at logon of this
  user with least privilege, created with `schtasks.exe`. Its definition is imported as XML so
  that it has no run-time limit, does not stop on battery, and restarts after a failure; the
  flags of `schtasks /Create` cannot set these.
- **FR-009** A daemon that `truss daemon start` started is never stopped or duplicated. When a
  daemon already answers at install time, the login item is installed but not started, and the
  output says it starts at the next login. (A second daemon exits with an error, and the
  restart policy would loop.) Otherwise install also starts it through the login mechanism.
  Reinstalling over an installation that is already running restarts that one to apply changes.

### Safety

- **FR-010** Install refuses a binary that looks temporary, with the reason and what to do:
  a `go run` or `go test` build directory (`go-build*`), the system temporary directory, `/tmp`,
  `/var/tmp`, `/var/folders` and Windows `AppData\Local\Temp` or `Windows\Temp`. Symbolic links
  are resolved first.
- **FR-011** Everything install creates carries the text `Managed by truss daemon install`.
  Install and uninstall leave alone, and report, a unit or plist at the same path without that
  text. Uninstall never removes anything else, and a task without the text is treated as not ours.
- **FR-012** A failed install does not leave a half-installed unit or agent behind: a file that
  install created is removed again. A failing `systemctl disable` during uninstall is reported
  as a warning and the unit file is removed anyway.
- **FR-013** Paths, addresses and the TRUSS home are quoted for the target format, so spaces,
  quotes, `%` and `$` in a path do not change the meaning of the generated file. A control
  character in any of them is refused.
- **FR-014** `TRUSS_HOME`, when set during install, is given to the daemon on Linux and macOS.
  On Windows install refuses it (the task cannot carry it); set the variable for your user
  instead.

## Environment

A daemon started at login has the login environment, not the environment of an interactive
shell profile. Steps see that environment plus the pipeline's `env`. The docs show, per
platform, how to give the daemon more: preferably `env` in the pipeline file.

## Out of scope

Starting the daemon from Desktop or from any other command; system-wide (all users) services;
installing on other init systems; a `--binary` override; passing the environment of the
installing shell into the login item.

## Contract notes (tier 2)

- Additive CLI change: two new `daemon` subcommands and one new line in `daemon status`. The
  existing lines and exit codes are unchanged. No change to the daemon API, the run log or the
  pipeline schema. `daemon.CheckAddress` is a new exported wrapper of the existing loopback check.
- Process execution: install starts only `systemctl`, `launchctl` or `schtasks.exe` with
  arguments built from fixed text and the flags above, and the daemon through them. No shell.

## Verification

- Generated files are pure functions of the `Spec`. Golden files in
  `internal/loginstart/testdata/` pin the unit, the plist, the task XML and the `schtasks`
  argument lists, with a plain and a hostile (spaces, `%`, `$`, `&`) input each.
- Install and uninstall run against a fake runner and an in-memory file system on every OS.
  They assert the exact command sequence, refusal of foreign files, rollback and idempotence.
- The CLI tests and the built-binary end-to-end script cover refusal of a temporary binary, flag
  errors and the status line, with the user's home redirected so no real login item is touched.
- **Not verified by running the real tools:** `systemctl --user`, `launchctl bootstrap` and
  `schtasks /Create /XML` were not run against a real systemd, launchd or Windows, and the task
  XML was not accepted by a real Task Scheduler. The first proof is a maintainer trying
  `truss daemon install` on each platform.
