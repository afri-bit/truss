# Run a flow on a schedule

Run a flow every night, every hour or on weekdays, even when you forget about it.

## 1. Add a schedule

```yaml
# .truss/flows/backup.yaml
version: "1"
flow:
  name: backup
on:
  schedule:
    - cron: "0 2 * * *"             # every day at 02:00
      timezone: Europe/Berlin       # optional; default: this machine's time
      missed: skip                  # optional; or run (catch up once)
steps:
  - id: copy
    run: rsync -a ~/photos/ /mnt/backup/photos/
```

`truss init` and `truss flows new` write the schedule for you from plain words ("every day at
02:00", "weekdays at 09:00").

| Cron | Means |
|---|---|
| `0 * * * *` | every hour |
| `0 2 * * *` | every day at 02:00 |
| `0 9 * * 1-5` | Monday to Friday at 09:00 |
| `*/15 * * * *` | every 15 minutes |
| `@daily` | every day at midnight |

The [flow file reference](pipeline-reference.md#onschedule) has the whole syntax.

## 2. Hand it to the daemon

```console
$ truss validate backup         # check it first
$ truss register --all          # the daemon learns the project's flows
$ truss daemon start            # the background service
$ truss schedules               # what runs next, and when
```

Make the daemon start when you log in, so schedules keep running after a restart:

```console
$ truss daemon install
```

## 3. Hear when it fails

```yaml
notify:
  desktop: true
```

See [notifications](guide-notifications.md) for Slack, Discord, Teams and email.

## The laptop question

A schedule fires only while the daemon runs. When the machine was off or asleep at that time:

- `missed: skip` (the default) runs nothing and records a skipped run;
- `missed: run` runs once as soon as the daemon is back, however many times were missed.

A missed schedule counts as a failure for `notify`, so you hear about it.

## Check on it

```console
$ truss runs backup             # the latest runs
$ truss runs show last          # steps, exit codes and a timeline of the newest run
$ truss logs <run-id>           # its log
$ truss pause backup            # stop its schedules for now; truss resume backup
```
