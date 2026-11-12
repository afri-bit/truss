# 0006. Start at login is per user, marker-owned and run without a shell

- Status: Proposed
- Date: 2026-11-12

## Context

The daemon does not survive a reboot. [Spec 011](../../specs/011-daemon-start-at-login/spec.md)
adds `truss daemon install` and `uninstall`. They change state outside the TRUSS home (a
systemd user unit, a launchd agent, a Task Scheduler task) and start processes, so the
boundaries need to be written down. The maintainer's rule stands: the daemon is the server,
started separately; Desktop and the CLI never start it implicitly.

## Decision

1. **Per user, never elevated.** Only the user's own unit directory, `LaunchAgents` folder and
   task are touched. Install refuses to run as root.
2. **Ownership by marker.** Everything install creates carries the text `Managed by truss daemon
   install`. Install, uninstall and status act only on items with it and report any other item
   of the same name as an error, without touching it. The Windows task name carries the user
   because the scheduler's root folder is machine-wide.
3. **No shell, three programs.** Install starts only `systemctl`, `launchctl` or `schtasks.exe`,
   with arguments built from fixed text and validated flags. File contents are pure functions
   of a `Spec`, pinned by golden files.
4. **A Windows task is imported as XML**, because `schtasks /Create` flags cannot turn off the
   72-hour run limit or stop-on-battery.
5. **A running daemon is never duplicated.** A second `daemon run` exits with an error, and the
   restart policies would loop. When a daemon answers, install writes the item but does not
   start it.
6. **A new package, `internal/loginstart`,** owns this. It knows nothing about the daemon or the
   API; `cmd/truss` decides whether a daemon runs and prints the messages it returns.

## Consequences

- Adding a platform means one more generator and one more branch in `Install`.
- Real `systemctl`, `launchctl` and `schtasks` behavior is covered by fakes only until a
  maintainer tries it on each system.
- A hand-edited unit, plist or task of the same name blocks install until the user moves it.
- On Windows, `uninstall` ends the task's daemon forcibly (`schtasks /End`); runs in progress
  are not cancelled gracefully.
