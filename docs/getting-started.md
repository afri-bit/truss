# Getting started

This page takes you from install to a scheduled pipeline in about five minutes. You need a
terminal and Go. TRUSS is one binary; nothing else runs until you ask.

## Install

With Go 1.27.3 or newer:

```bash
go install github.com/afri-bit/truss/cmd/truss@latest
truss version
```

Or build from source:

```bash
git clone https://github.com/afri-bit/truss.git
cd truss
go build -o bin/truss ./cmd/truss
```

Prebuilt binaries will be published on [GitHub Releases](https://github.com/afri-bit/truss/releases)
with the first tagged version.

## Make a project

A project is a folder with a `truss.yaml` file. It names the project and holds nothing that runs.

```yaml
# truss.yaml
version: "1"

project:
  name: demo
```

Pipelines live next to it, one file each, in `.truss/flows/`:

```yaml
# .truss/flows/hello.yaml
version: "1"

pipeline:
  name: hello

steps:
  - id: greet
    run: echo "Hello from TRUSS"
```

## Run a pipeline

Run it by name, from the project folder:

```console
$ truss run hello
2026-11-16 09:00:00 setup environment ready
2026-11-16 09:00:00 setup os linux
2026-11-16 09:00:00 setup arch amd64
2026-11-16 09:00:00 setup shell /bin/sh
2026-11-16 09:00:00 setup directory /home/me/demo
2026-11-16 09:00:00 step greet started
2026-11-16 09:00:00 step greet | Hello from TRUSS
2026-11-16 09:00:00 step greet finished duration 0.006s exit 0
2026-11-16 09:00:00 summary succeeded steps 1 duration 0.006s
```

`truss run` with no name is an error that lists the pipelines it finds (exit 1). `truss run <path>` runs any pipeline file.

TRUSS keeps what a run leaves behind in `.truss/state/<name>/`: the run log, and the data and cache
folders your steps can use. It also writes `.truss/.gitignore` so that only `state/` is ignored and
your pipelines stay in git.

## Add a pipeline with dependencies

Group steps into jobs and let a job wait for others. Jobs that do not depend on each other run side
by side.

```yaml
# .truss/flows/ci.yaml
version: "1"

pipeline:
  name: ci
  concurrency: 2

jobs:
  lint:
    steps:
      - id: vet
        run: echo "vet ok"
  test:
    steps:
      - id: unit
        run: echo "tests ok"
  release:
    needs: [lint, test]
    steps:
      - id: build
        run: echo "building"
```

Check the files without running anything:

```console
$ truss validate
truss.yaml is valid: project demo
.truss/flows/ci.yaml is valid: pipeline ci, 3 steps, concurrency 2
step lint/vet
step test/unit
step release/build needs lint/vet, test/unit
.truss/flows/hello.yaml is valid: pipeline hello, 1 steps, concurrency 1
step greet
```

`lint` and `test` run together. `release` starts when both have succeeded. The first failed step
fails the run and stops the rest.

## Let the daemon run it

The daemon is a small background service with a queue, a history and one API for every client. Start
it, tell it about your pipelines, and trigger one:

```console
$ truss daemon start
daemon started (pid 4242, 127.0.0.1:7878)

$ truss register --all
registered ci (/home/me/demo/.truss/flows/ci.yaml)
registered hello (/home/me/demo/.truss/flows/hello.yaml)
registered 2 pipelines (0 with schedules)

$ truss trigger ci --follow
...
2026-11-16 09:02:00 summary succeeded steps 3 duration 0.029s

$ truss runs
RUN                     PIPELINE  STATUS     TRIGGER  CREATED              DURATION  NOTE
20261116-090200-5c121e  ci        succeeded  manual   2026-11-16 09:02:00  0s
```

The daemon does not start by itself after a reboot. Run `truss daemon install` once to start it at
every login (`truss daemon uninstall` takes it back). Details are in [the daemon docs](daemon.md).

## Put it on a schedule

Add a trigger to the pipeline file and save. The daemon re-reads the files it already knows, so a change to a registered pipeline takes effect by itself.

```yaml
# .truss/flows/nightly.yaml
version: "1"

pipeline:
  name: nightly

on:
  schedule:
    - cron: "30 1 * * *"
      missed: run   # catch up once if the machine was off

steps:
  - id: report
    run: echo "nightly report"
```

A job planned for 01:30 does not fire at 11:00 because you opened the lid late, unless you ask for a
catch-up run with `missed: run`. Register the new file with `truss register nightly`, and
`truss pipelines` shows its next run.

## Open TRUSS Desktop

```console
$ truss desktop
```

Desktop is a dashboard in your browser. It shows your pipelines, every run as a step graph, and the
live log. It talks to the daemon, so start the daemon first.

## Where next

- [Pipeline file reference](pipeline-reference.md): every field, cron syntax and recipes.
- [Command line reference](cli.md): every command and exit code.
- [The daemon](daemon.md): registration, schedules, missed runs and concurrency.
- [TRUSS Desktop](desktop.md): what the dashboard shows.
- [Glossary](GLOSSARY.md): what each word means here.
