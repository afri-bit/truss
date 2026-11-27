# Troubleshooting

Find your symptom, then the cause and the fix.

## Running

| Symptom | Cause | Fix |
|---|---|---|
| `error: the TRUSS daemon is not running` | The command needs the daemon. | `truss daemon start`, or `truss daemon install` to start it at login. |
| `no flow named "x"` | The name is not a file in `.truss/flows/`, or you are not in the project folder. | `truss flows` lists them; run from the folder with `truss.yaml`, or give the file's path. |
| A step fails with `command not found` under the daemon but works in your terminal | The daemon has a shorter `PATH`. | Use full paths, or set `PATH` in `env`: `PATH: /usr/local/bin:${PATH}`. See [the environment of a login-started daemon](daemon.md#the-environment-of-a-login-started-daemon). |
| `dir uses ${NAME}, which is not set` | A `dir` uses a variable that has no value. | Set it in `env`, or check its name with `truss flows show <flow> <step>`. |
| A step `timed out after ...` | It ran longer than its `timeout`. | Raise `timeout`; `truss runs show last` shows how long steps took. |
| A secret is `missing` | It is not in the store. | `truss secret set NAME`. |
| `X is set by TRUSS in every run and cannot be set here` | A built-in name in `env` or `secrets`. | Pick another name; [built-in variables](pipeline-reference.md#built-in-variables). |

## Schedules and hooks

| Symptom | Cause | Fix |
|---|---|---|
| A schedule never runs | The flow is not registered, or paused, or the daemon is stopped. | `truss schedules` shows what is armed; `truss register --all`; `truss resume <flow>`. |
| A run shows `skipped` with "missed" | The machine was off or asleep at that time. | `missed: run` on the schedule catches up once. |
| An `on.file` flow never starts | The paths match nothing, the flow is paused, or the daemon is stopped. | `truss flows show <flow>` lists the hook; `truss daemon logs` says when a scan stopped at a limit or skipped a link. |
| An `on.file` flow keeps starting itself | It writes into the folder it watches. | Write elsewhere, or add the output to `ignore`. |

## Notifications

| Symptom | Cause | Fix |
|---|---|---|
| No desktop notification on Linux | `notify-send` is missing or there is no desktop session. | Install `libnotify`; `truss daemon logs` shows the error. |
| No Slack message | The secret is not set, or the address is wrong. | `truss secret list`; `truss run <flow> --notify` shows the error. |

## Where to look

```console
$ truss status                        # project, daemon, flows, latest run
$ truss runs show last                # steps, exit codes, timeline
$ truss logs <run-id>                 # a run's log
$ truss daemon logs --follow          # what the daemon did, live
$ truss daemon config                 # its settings and where they come from
$ truss env                           # where TRUSS keeps its files
```

If you think TRUSS is wrong, open an issue with the output of `truss version`, `truss status` and
the flow file: https://github.com/afri-bit/truss/issues
