# What TRUSS is

TRUSS runs your workflows on your own machine: by hand, on a schedule, when files change, or after
another workflow, and it remembers every run. A workflow is a **flow**: a YAML file of steps, each
a command for the system shell (`sh`, or `cmd` on Windows).

```yaml
# .truss/flows/backup.yaml
version: "1"
flow:
  name: backup
on:
  schedule:
    - cron: "0 2 * * *"           # every night at 02:00
notify:
  desktop: true                   # tell me when it fails
steps:
  - id: copy
    run: rsync -a ~/photos/ /mnt/backup/photos/
```

## What you get

- **Run here, now.** `truss run backup` runs the flow here, prints a readable log and keeps it.
- **Run on its own.** A small background service, the [daemon](daemon.md), starts flows on a
  schedule, when files change, or after another flow, and keeps their history.
- **Know what happened.** `truss runs`, `truss status`, a terminal dashboard and
  [TRUSS Desktop](desktop.md) in your browser show every run, step and exit code; a failure can
  notify you.
- **Fix it fast.** `truss flows show backup copy` shows a step's command and everything it can use;
  `truss run backup --step copy` runs just that step.

## When TRUSS fits, and when something else does

| You want to | Use |
|---|---|
| Run a build, a sync, a backup or a report on your machine, by hand or on a schedule, and see how it went | TRUSS |
| Build and test on every push, on GitHub's or Jenkins' machines | Your CI. It can call `truss run` so the same flow runs on your laptop and in CI. |
| Run GitHub Actions workflows locally | [act](https://github.com/nektos/act) |
| A list of commands for a project, without history or schedules | Make or [Task](https://taskfile.dev). A TRUSS step can call `make` or `task`. |
| One command at a fixed time, nothing else | cron or a systemd timer |

## The words

| Word | Meaning |
|---|---|
| **Project** | A folder with a `truss.yaml`. Its flows are files in `.truss/flows/`. |
| **Flow** | One YAML file: what to run and when. Older text says "pipeline". |
| **Step** | One command of a flow. Steps can depend on each other (`needs`). |
| **Job** | A named group of steps that run one after another. |
| **Run** | One execution of a flow, with its log and result. |
| **Trigger** | Why a run started: `manual`, `schedule`, `file`, `run`, `local`, `catch-up`. |
| **Daemon** | The background service that starts flows and keeps their history. |

The [glossary](GLOSSARY.md) has every term.

## Where to read next

| To | Read |
|---|---|
| Set up a project in five minutes | [Getting started](getting-started.md) |
| Do one thing (a schedule, a hook, a notification, variables) | The guides: [schedules](guide-schedules.md), [hooks](guide-hooks.md), [notifications](guide-notifications.md), [variables and secrets](guide-variables.md), [Windows, macOS and Linux](guide-platforms.md) |
| Understand how a run works | [How a run works](how-a-run-works.md) |
| Look up a field or a command | [Flow file reference](pipeline-reference.md), [command line](cli.md), [the daemon](daemon.md) |
| Fix a problem | [Troubleshooting](troubleshooting.md) |
