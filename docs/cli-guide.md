# Using the command line

This page is a tour of the `truss` command: how the commands are organised, what to type to answer
common questions, and how to use them in scripts. The [command line reference](cli.md) lists every
command and option.

## How the commands are organised

Commands come in layers: a **noun**, then what to do with it. You can always ask `truss help` for the
list, and `truss help <command>` for one command.

```text
truss
├── status                      is everything fine?
├── dashboard                   the same, as a live screen
├── project / projects          this folder's project / every project the daemon knows
├── flows  list | show | graph  the pipeline files of this project
├── pipelines  list | show      what the daemon has registered
├── schedules                   what runs when
├── runs  [<pipeline>] | show   what ran, and how it went
├── daemon  start | stop | status | logs | install | uninstall | run
├── desktop                     open TRUSS Desktop
├── run / validate              run or check a pipeline here, without the daemon
├── register / unregister / pause / resume / trigger / cancel / logs
├── secret  set | list | remove
├── env                         where TRUSS keeps its files
└── help [<command>] | version
```

Three questions decide which command to use:

| You want to know about | Ask | Needs the daemon |
|---|---|---|
| The files in this project | `truss flows`, `truss project`, `truss validate` | no |
| What the daemon is doing | `truss pipelines`, `truss schedules`, `truss runs`, `truss projects` (`truss status` and `truss dashboard` also work without one) | yes |
| Where TRUSS keeps things | `truss env`, `truss daemon logs` | no |

Commands that can answer without a daemon do, and say `unknown` for what only the daemon knows. The
read-only commands change nothing and never start or stop the daemon.

## Is everything fine?

```console
$ truss status
TRUSS      0.1.0 (linux/amd64)
Project    shop, 4 flows in .truss/flows/
Daemon     running, pid 4121, up 2h 5m, 127.0.0.1:7878
Desktop    http://127.0.0.1:7878/ (open it with: truss desktop)
Runs       1 running, 0 queued (limit 4 at once)
Flows      4 registered: 3 ready, 1 running, 0 paused, 0 invalid
Last run   build running just now (truss runs show 20261117-091500-a1b2c3)
Home       /home/me/.truss
```

Without a daemon the screen says `Daemon     not running (start it with: truss daemon start)`.
`truss status` exits `0` either way, because it answers a question. When a script needs to know
whether the daemon runs, use `truss daemon status`, which exits `1` when it does not.

## The dashboard

`truss dashboard` draws the same facts as a frame of plain ASCII, with the load of the machine and
the latest runs. On a terminal it redraws every two seconds until you press Ctrl-C.

```text
+- TRUSS 0.1.0  2026-11-17 09:15:02 -------------------------------------------+
| Daemon   running  pid 4121  127.0.0.1:7878  up 2h 5m                         |
| Runs     1 running, 0 queued   [#####...............] 1 of 4 slots           |
| Host     CPU [##........]  21%   Mem [######....]  61% (9.6G of 15.7G)       |
+------------------------------------------------------------------------------+
+- Flows (3) ------------------------------------------------------------------+
| STATE    FLOW     NEXT RUN             LAST RUN                              |
| running  build    -                    succeeded 14m 2s ago                  |
| ready    nightly  2026-11-18 01:30:00  failed 23h 40m ago                    |
| paused   release  -                    -                                     |
+------------------------------------------------------------------------------+
+- Latest runs ----------------------------------------------------------------+
| RUN                     FLOW   STATUS     WHEN        TOOK                   |
| 20261117-091500-a1b2c3  build  running    just now    12s                    |
| 20261117-090100-9f8e7d  build  succeeded  14m 2s ago  41s                    |
+------------------------------------------------------------------------------+
```

- The frame is as wide as `COLUMNS` (60 to 120, default 80). Export `COLUMNS` to get a wider screen.
- It uses `+ - |`, `#` and `.` only: no colour and no special font, so it reads the same over SSH, in a
  log and in a CI job.
- `--once`, a pipe, a file and the Windows console draw it once instead of redrawing.
- Without a daemon it says so and lists the flows of your project.

## Looking around a project

```console
$ truss flows
FLOW     STEPS  SCHEDULES  DAEMON          FILE
build    6      0          registered      .truss/flows/build.yaml
nightly  3      1          registered      .truss/flows/nightly.yaml
release  4      0          not-registered  .truss/flows/release.yaml
```

`DAEMON` says whether the daemon has the flow (`registered`, `paused`, `not-registered`), or
`unknown` when no daemon runs. A flow whose file does not load shows `INVALID:` and its error; the
other flows are still listed.

```console
$ truss flows show nightly
Flow          nightly
File          .truss/flows/nightly.yaml
Project       shop
Daemon        registered
Concurrency   2
Overlap       queue
Environment   REGION, STAGE (names only)
Secrets       NPM_TOKEN (names only)
Schedule      30 1 * * * (UTC, missed: run)

STEP     NEEDS         TIMEOUT  RETRIES  IF  RUN
compile  -             10m0s    -        -   make build
test     compile       -        2        -   make test
publish  compile,test  -        -        -   make publish
```

Environment variable and secret **names** are listed so you can see what a flow depends on. Values
are never printed here; to see them for one step, use [`flows show <flow> <step>`](#debugging-a-step). `flows graph` shows the order the steps run in:

```console
$ truss flows graph nightly
nightly: 3 steps in 3 stages, at most 2 at once

stage 1
  [compile]

stage 2
  [test]  <- compile

stage 3
  [publish]  <- compile, test
```

Steps in one stage do not wait for each other, so they can run together up to the pipeline's
`concurrency`. To see the project itself, use `truss project`; to see every project the daemon has
been told about, use `truss projects`:

```console
$ truss projects
PROJECT  FLOWS  RUNNING  PAUSED  INVALID  ROOT
shop     4      1        0       0        /home/me/work/shop
docs     2      0        1       0        /home/me/work/docs
```

## Debugging a step

A flow fails at step 9 of 12. You do not want to run all of it again. Look at the step first,
then run only that step.

```console
$ truss flows show ci build/package
Flow          ci
File          .truss/flows/ci.yaml
Step          build/package
Job           build
Needs         build/compile
Needed by     deploy/push
Directory     /home/me/work/shop
Timeout       -
Retries       0
If            -
Command
  echo "packaging for $REGION/$STAGE"
  npm publish --registry "$REGISTRY_URL"

NAME             SOURCE            STATE        VALUE
REGION           pipeline          set          eu
STAGE            step              set          staging
NPM_TOKEN        secret store      missing      -
TRUSS_DATA_DIR   truss run         set          .truss/state/ci/data
TRUSS_CACHE_DIR  truss run         set          .truss/state/ci/data/cache
TRUSS_RUN_ID     daemon            daemon only  -
TRUSS_PIPELINE   daemon            daemon only  -
TRUSS_TRIGGER    daemon            daemon only  -
REGISTRY_URL     host environment  not set      -
```

The table lists what the step can use and where each value comes from: your file (`pipeline`,
`job <id>` or `step`, whichever layer wins), the secret store, TRUSS itself, or your shell
(`host environment`). Here two things are wrong before anything runs: the secret `NPM_TOKEN` is
`missing`, and the command reads `$REGISTRY_URL`, which nothing sets.

- A secret shows `set` and `***`, or `missing`. Its value is never printed.
- A variable whose name has `TOKEN`, `SECRET`, `PASSWORD`, `KEY`, `AUTH` or a similar word shows
  `***`, wherever it comes from, so a screenshot is safe.
- The `host environment` rows come from reading the command text for `$NAME`, `${NAME}`,
  `%NAME%` and `$env:NAME` with upper case names. A script can read variables this does not find.
  Add `--values` to see their values (`***` for token-like names and stored secrets).
- `TRUSS_RUN_ID`, `TRUSS_PIPELINE` and `TRUSS_TRIGGER` are set only by the daemon. To try a step
  as if the daemon ran it, give them with `--env`.

`truss flows show ci build/package --json` prints the same as JSON.

Now run only that step. The steps it needs are not run, and the step starts as if they had
succeeded:

```console
$ truss run ci --step build/package --env REGISTRY_URL=https://registry.example.com
debug run: 1 of 3 steps (build/package); the run log is not saved
2026-11-19 20:58:29 ==== Setup: the environment the steps run in ====
...
2026-11-19 20:58:29 ==== Step build/package ====
2026-11-19 20:58:29 step build/package started
2026-11-19 20:58:29 step build/package | packaging for eu/staging
```

| Option | What it runs |
|---|---|
| `--step <id>` | Only that step. |
| `--step <id> --with-needs` | That step and every step it needs, directly or not. |
| `--from <id>` | That step and every step that needs it, directly or not. |
| `--env NAME=VALUE` | Sets or replaces a variable for the steps that run. Repeat it for more. A secret is refused: use `truss secret set`. |

A debug run prints its log but does not save it: the `run.log` of your last real run stays. It uses
the flow's data and cache folders, so files an earlier run left are still there. It keeps the
step's settings (`timeout`, `retries`, `dir`, `env`, `secrets`) and its own `if`; the `if` of a
job of the step you named is not evaluated, because you named it; other jobs keep theirs, so
`--from` never starts a deploy the flow would skip. An `--env` value is seen by `if` conditions,
like any variable of the step. It does not combine with `--track`.

When a `truss run` fails, TRUSS tells you the command for the failed step, on standard error:

```console
$ truss run ci
...
to run only this step again: truss run ci --step build/package
```

The hint does not repeat your `--env` options; add them. It is printed only when the flow and the
step id are plain words (letters, digits and `_ . / : @ + = , -`), because quoting differs between
shells and a step id can be any text.

An `--env` value is an argument, so other users of the machine can see it in the process list. Put
credentials in the secret store (`truss secret set`) and name them in `secrets:`; `--env` refuses a
name a step declares as a secret.

## What is scheduled

```console
$ truss schedules
NEXT RUN             PIPELINE  CRON         ZONE   MISSED  STATE
2026-11-17 09:20:00  poll      */5 * * * *  local  skip    active
2026-11-18 01:30:00  nightly   30 1 * * *   UTC    run     active
```

The list is across every registered pipeline, soonest first. A paused pipeline still shows its
schedules, marked `paused`. `truss pipelines show <name>` gives one pipeline in full: its file,
schedules, steps and five latest runs.

## How did that run go?

Find runs, then look at one:

```console
$ truss runs --status failed --limit 5
$ truss runs show last
Run        20261117-091500-a1b2c3
Pipeline   nightly
Status     failed: nightly failed at test (exit 3)
Trigger    schedule
Created    2026-11-17 09:15:00 (4m 2s ago)
Took       12s
Steps      3 (1 succeeded, 1 failed, 1 stopped)
Log        truss logs 20261117-091500-a1b2c3

STEP     STATUS     EXIT  TOOK  TRY  TIMELINE
compile  succeeded  0     4.2s  -    |#######...............|
test     failed     3     7.1s  2    |.......##############.|
publish  stopped    -     -     -    |......................|
```

The timeline shows when each step ran inside the run, so a slow step or a gap (a step waiting for
the concurrency limit) is easy to see. `TRY` is the attempt number of a step that was retried. Steps
that never started and steps that failed are explained under `NOTES`. Then read the log:

```console
$ truss logs 20261117-091500-a1b2c3
```

`truss runs show` exits `0` even for a failed run: it reports. To make a command fail when a run
fails, use `truss trigger <name> --follow`, which exits `1` unless the run succeeded.

| Option of `truss runs` | Meaning |
|---|---|
| `<pipeline>` | Only runs of that pipeline. |
| `--status <s>` | `queued`, `running`, `succeeded`, `failed`, `cancelled` or `skipped`. |
| `--active` | Queued or running runs. |
| `--limit <n>` | At most `n` runs (default 30). |

`--status` and `--active` look at the newest 1000 runs.

## The daemon and its log

```console
$ truss daemon status
$ truss daemon logs -n 20
$ truss daemon logs --follow
```

The daemon's own log records when it started, which files it registered, and which runs it started
for which reason. It is a file, so `truss daemon logs` works when the daemon has crashed or stopped,
which is when you want it. `truss env` shows where that file is, with the TRUSS home and the project
it sees from here.

## Scripting

Add `--json` to `status`, `project`, `projects`, `flows`, `flows show`, `pipelines`,
`pipelines show`, `schedules`, `runs`, `runs show` and `daemon status`. One indented JSON document
goes to standard output. Lists are `[]` when empty, never `null`. Daemon objects (`pipelines`,
`runs`) use the field names of the [daemon API](daemon.md); the other documents are described below.

```bash
# fail a script when the daemon is down
truss daemon status --json >/dev/null || { echo "daemon down" >&2; exit 1; }

# names of the pipelines whose last run failed
truss pipelines --json | jq -r '.[] | select(.last_run.status == "failed") | .name'

# the newest failed run of one pipeline
truss runs nightly --status failed --limit 1 --json | jq -r '.[0].id'

# flows of this project the daemon does not have yet
truss flows --json | jq -r '.[] | select(.daemon == "not-registered") | .name'
```

| Command | JSON |
|---|---|
| `status` | `{version, project, daemon, desktop_url, pipelines: {registered, ready, running, paused, invalid}, recent_runs, home}`. `project`, `daemon` and `desktop_url` are `null` outside a project or without a daemon; `project_error` and `daemon_error` appear when the cause is a problem (an unreadable `truss.yaml`, a daemon that does not answer), and `status` still exits `0`. |
| `daemon status` | `{running, daemon}`. Exit `1` when not running. |
| `project` | `{name, description, root, flows, registered}`. `registered` is absent without a daemon. |
| `projects` | `[{name, root, registered, running, paused, invalid}]`. |
| `flows` (a list of these), `flows show` | `{name, file, path, valid, error, pipeline, daemon, concurrency, overlap, jobs, schedules, env, secrets, steps}`; `env` and `secrets` are names only, `daemon` is one of `registered`, `paused`, `not-registered`, `unknown`. |
| `schedules` | `[{pipeline, cron, timezone, missed, next, paused}]`. |
| `pipelines`, `pipelines show`, `runs`, `runs show` | The objects of the [daemon API](daemon.md). |

Exit codes are the same everywhere: `0` on success, `1` on any error, with a one-line
`error: ...` on standard error. Output meant for a script goes to standard output, hints and errors
to standard error.

## Finding your way

- `truss help` lists every command; `truss help flows` or `truss flows --help` explains one.
- A command that needs the daemon says `start it with: truss daemon start` when none runs.
- A command that needs a project and finds no `truss.yaml` names the folder it looked in. Run TRUSS
  from the project folder.
- A flow name that is not found lists the flows that exist; an unknown pipeline or run id points to the command that lists them.
- `truss run --help` and `truss validate --help` are not help. Their argument is a flow name or a path.

## Safe to read

These commands are made for looking at flows before you run them, including flows you did not
write. A control character in a file (an escape sequence, a bell, a carriage return) is shown as `?`
instead of reaching your terminal, and a `run` of several lines shows `(+N lines)` after its first
line. `--json` has the exact text, escaped. Wide characters (CJK, emoji) can still misalign a frame.

## Compatibility

The commands of earlier versions keep their names, options, output and exit codes. New words
(`show`, `list`, `logs`) are only taken where the old command would have printed usage, so a script
that worked keeps working. A pipeline named `show` still works: `truss runs show` lists its runs.
