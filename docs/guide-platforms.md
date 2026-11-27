# Windows, macOS and Linux

TRUSS runs on all three. A step's `run` is a command for the system's shell, so a flow written for
one shell may need changes for another.

| | Linux | macOS | Windows |
|---|---|---|---|
| Shell of a step | `sh -c` | `sh -c` | `cmd /C` |
| A variable in `run` | `$NAME` | `$NAME` | `%NAME%` |
| `${NAME}` in `env` and `dir` | the same everywhere (TRUSS replaces it) | | |
| Stopping a step | SIGTERM, then SIGKILL after `kill_timeout` | the same | ended at once |
| Variable names | case matters | case matters | case does not matter |
| Daemon at login | systemd user unit | launchd agent | Task Scheduler |
| Desktop notifications | `notify-send` | Notification Center | toast |

## Write a flow that runs everywhere

- Keep commands that differ in a script per system, or call a tool that is the same everywhere
  (`go`, `npm`, `python`, `make`).
- Use `${NAME}` in `env` and `dir`: TRUSS replaces it, whatever the shell.
- Use `/` in `on.file` paths on every system.
- `truss init` writes a flow with commands for the system it runs on.

## Windows

- `cmd` keeps a space before `&&` in `echo` output: write `echo %A%&& echo %B%`.
- A step's processes end at once when it is stopped; `kill_timeout` does not apply.
- Desktop notifications need a logged-in user (Windows PowerShell 5.1 shows them).

## macOS

- A daemon started at login has a shorter `PATH` than your terminal; see
  [the environment of a login-started daemon](daemon.md#the-environment-of-a-login-started-daemon).
- Notifications may be held back by Focus or by the notification settings of Script Editor.

## Linux

- Desktop notifications need `notify-send` and a desktop session.
- `truss daemon install` uses a systemd user unit; `journalctl --user -u truss` shows its output.
