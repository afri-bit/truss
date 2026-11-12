# Feature Specification: Daemon start at login

**Spec**: `011-daemon-start-at-login` | **Created**: 2026-11-12 | **Status**: Implemented (generation and fake-runner tests only; see Verification)

**Input**: "The daemon does not start at login, so after a reboot nothing runs until the user
starts it." The rule from [006](../006-daemon-registry-and-scheduling/spec.md) stands: the daemon
is the server, started separately. Desktop and the CLI never start it implicitly.

Decision: [ADR 0008](../../docs/decisions/0008-login-start-is-per-user-and-marker-owned.md).

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
  When the state cannot be read (for example a file of the same name that install did not
  create) the line says `unknown`. On an unsupported system, or when the home directory cannot
  be found, the line is left out. Status never fails because of it, and the check is bounded to
  five seconds.
- **FR-004** Installing again replaces the earlier installation (new binary path or flags) and
  says `updated`.
- **FR-005** Nothing runs with root or administrator rights, and nothing is written outside the
  user's own directories or per-user scheduler. Install refuses when run as root.

### Per platform

- **FR-006 Linux.** A systemd user unit `~/.config/systemd/user/truss.service` with
  `ExecStart=<binary> daemon run [flags]`, `Restart=on-failure`, `WantedBy=default.target`,
  enabled with `systemctl --user`. The output mentions `loginctl enable-linger` for running
  without a login session.
- **FR-007 macOS.** A launchd agent `~/Library/LaunchAgents/io.github.afri-bit.truss.plist` with
  `RunAtLoad` and `KeepAlive` on failure only (`SuccessfulExit` false), output going to
  `daemon.log` in the TRUSS home, loaded into the `gui/<uid>` domain with `launchctl`.
- **FR-008 Windows.** A per-user Task Scheduler task `truss-daemon-<user>` (the scheduler's
  root folder is shared by all accounts, so the name carries the user) that runs at logon of this
  user with least privilege, created with `schtasks.exe`. Its definition is imported as XML so
  that it has no run-time limit, does not stop on battery, and restarts after a failure; the
  flags of `schtasks /Create` cannot set these.
- **FR-009** A daemon that `truss daemon start` started is never stopped or duplicated. When a
  daemon already answers at install time, the login item is installed but not started, and the
  output says it starts at the next login. (A second daemon exits with an error, and the
  restart policy would loop.) Otherwise install also starts it through the login mechanism.
  On Linux, reinstalling over a service that is running restarts it to apply the change. On
  macOS and Windows a running daemon keeps its old settings; the output says to run
  `truss daemon stop` and install again. If install cannot tell whether a daemon runs (an
  answer other than "not running") it fails instead of guessing.

### Safety

- **FR-010** Install refuses a binary that looks temporary, with the reason and what to do:
  a `go run` or `go test` build directory (`go-build*`), the system temporary directory, `/tmp`,
  `/var/tmp`, `/var/folders` and Windows `AppData\Local\Temp` or `Windows\Temp`. Symbolic links
  are resolved first.
- **FR-011** Everything install creates carries the text `Managed by truss daemon install`.
  Install and uninstall leave alone, and report as an error, a unit, plist or task of the same
  name without that text. Uninstall never removes anything else. Files are written beside
  their target and renamed into place, so a crash leaves the old or the new file.
- **FR-012** A failed install does not leave a half-installed unit or agent behind: a unit or
  plist that install created is removed again (and a new unit disabled), including when the
  start after enabling fails. A Windows task that was created but could not be started stays,
  and the error says so. An agent unloaded for a reload that then fails to load again stays on
  disk and the error says how to load it. A failing `systemctl disable` during uninstall is reported
  as a warning and the unit file is removed anyway.
- **FR-013** Paths, addresses and the TRUSS home are quoted for the target format, so spaces,
  quotes, `%` and `$` in a path do not change the meaning of the generated file. A control
  character or invalid UTF-8 in any of them is refused. Task Scheduler expands `%NAME%` in a
  command and offers no escape, so on Windows a binary path containing `%` is refused. The
  `--address` port must be a number from 0 to 65535.
- **FR-014** `TRUSS_HOME`, when set during install, is given to the daemon on Linux and macOS.
  A Windows task cannot carry it: install proceeds and says that the daemon uses the user
  variable of that name (`setx`) or the default home.

## Environment

A daemon started at login has the login environment, not the environment of an interactive
shell profile. Steps see that environment plus the pipeline's `env`. The docs show, per
platform, how to give the daemon more: preferably `env` in the pipeline file.

## Known limits

- Windows: `uninstall` ends the task's daemon with `schtasks /End`, which does not cancel runs
  in progress gracefully. Run `truss daemon stop` first.
- Windows: the daemon runs in a console window that stays open and its output is not saved.
  Creating a logon task may need elevation on some setups; install then reports the error and
  nothing is changed. Neither is verified on a real machine.
- A reload of a running macOS agent retries `launchctl bootstrap` up to five times, because it
  can fail briefly right after `bootout`.

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
- Further tests: rollback when `restart` or `/Run` fails, refusal when elevated, per-user task
  names, Windows argument quoting, atomic file replacement, and the CLI wiring
  (`installLogin`) with a fake host.
- **Not verified by running the real tools:** `systemctl --user`, `launchctl bootstrap` and
  `schtasks /Create /XML` were not run against a real systemd, launchd or Windows, and the task
  XML was not accepted by a real Task Scheduler. The first proof is a maintainer trying
  `truss daemon install` on each platform.
