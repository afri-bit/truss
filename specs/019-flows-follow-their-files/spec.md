# Feature Specification: Flows follow their files

**Spec**: `019-flows-follow-their-files` | **Created**: 2026-11-16 | **Status**: Implemented

Extends [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md) (the
file stays the source of truth) and [`016-project-layout`](../016-project-layout/spec.md) (a
pipeline's name is optional and defaults to the file name). User docs:
[`docs/daemon.md`](../../docs/daemon.md#registering-pipelines). Decision:
[ADR 0017](../../docs/decisions/0017-a-registration-keeps-its-name-the-file-names-it.md).
Risk class: tier 2 (daemon API and what a name means to clients). Scope decided by the maintainer on
2026-11-16 (answers: follow the file and keep the history; keep polling, no new dependency).

## User stories

1. **Edit, save, done** (P1). I change a registered pipeline file (its steps, its schedule or its
   `pipeline.name`) and save. TRUSS Desktop and the CLI show the file as it is now, and the next run
   runs it as it is now. I restart nothing and register nothing again.
2. **Keep the history** (P1). When the name changes, the runs the flow already has stay with it. They
   do not move to a flow that no longer exists, and nothing is lost.
3. **Old scripts keep working** (P2). A script that triggers the flow by the name it was registered
   under still works after the file renames it.

## Requirements

- **FR-001** The daemon reads a registered file again when its size or modification time changed.
  It checks every second and also on every request that shows or uses the file, so no answer lags an
  edit by up to a second (a request trusts a look at a file for at most 250 ms). A file that no
  longer loads is shown as invalid with the reason, and its history stays. It is read again at every
  check until it loads, so a read that failed for a passing reason heals without another edit. A
  file that fails on its content (YAML or validation) fails the same way until it changes and is
  not parsed again.
- **FR-002** Each registration has a **registered name**: the name it was registered under. It never
  changes. Run history, schedules, overlap control and the data and cache folders hang on it.
- **FR-003** The name the daemon **shows** for a registration is `pipeline.name` from its file, when
  that is a valid registration name and no other registration holds or claims it. Otherwise it is the
  registered name. A name chosen at registration (`--name`, or the name field in Desktop) is never
  replaced by the file's. A file that stops loading keeps the name it was last read with, so a name
  does not move while someone fixes the file. One name never answers for two registrations: a registered name always wins, and
  when two files claim the same new name, both keep their registered names.
- **FR-004** `name` in the API's pipeline object is the shown name. `registered_name` is present only
  when it differs. `definition_name` is unchanged.
- **FR-005** Every command and API call that takes a pipeline name accepts the registered name or the
  shown name (a name that answers to nothing is looked up once more with a fresh look at the files,
  so a name a file took a moment ago is found): `GET`, `DELETE` and the `pause`, `resume` and `runs` calls of `/pipelines/{name}`, and
  `GET /runs?pipeline=`. A name that answers to no registration is an error as before (`404`).
- **FR-006** The history of a flow is every run of its registered name, plus any run recorded under
  its shown name (a run tracked from a shell reports the file's name). `GET /runs?pipeline=` and the
  pipeline's `last_run` and `active_runs` use it.
- **FR-007** A run's `pipeline` is the shown name of its registration, in every response, and
  `registered_pipeline` is the registration's name when that differs. Use it to act on the
  registration: a shown name can move to another flow, a registered name cannot. A run record on
  disk keeps the registered name; nothing in the run history is rewritten.
- **FR-008** Registering a file without `--name` (as `truss register --all` does) returns the
  registration that file already has, whatever name it carries or shows (even one another
  registration holds: no `409`), and creates no second one. A request that creates nothing writes no
  "registered" line to the log.
  `--name` still makes another registration of the same file under that name. Registering another
  file under a name a registration holds or shows is `409`, as before. Registering a file again with
  `--name` set to its registered name marks that name as chosen.
- **FR-009** The daemon log has a line when a registered file changed and was read again (also when
  a run trigger is the first to see the change), with the reason when it no longer loads and the
  file's name when it differs from the registered name. A file that stays broken is logged once.
- **FR-010** TRUSS Desktop shows the registered name on the flow's page, as *Registered as*, when it
  differs. Lists, the run page and *Jump to…* use the shown name, and the page follows an edit by
  itself. Its buttons (run now, pause, resume, unregister, re-run) act on the registration, by its
  registered name. `GET /pipelines` lists flows sorted by the name shown.

## Not in this spec

Changing a registration's registered name (to do so, unregister and register again; the history stays
with the old name); history of more than the registered name and the shown name (a flow renamed
twice finds only runs tracked under its current name and runs started by the daemon); watching with
file-system events (no new dependency; one second is enough); moving data or cache folders;
`TRUSS_PIPELINE` in a step's environment, which stays the registered name so a run's environment
does not change under a flow.

## Known limits

- **Scripts should use the registered name.** A shown name can move: if the file that holds it stops
  claiming it, or another file starts to, it answers to a different flow or to none. A registered
  name never moves.
- **History can take in runs of the same name.** A flow's history includes any run recorded under
  its shown name: a run tracked from a shell (`truss run --track`) of an unrelated file with that
  name, or old runs of a flow that was unregistered under that name. Scheduling and overlap control
  use the registered name only, so this changes what is shown, not what runs.
- **Network and synced folders.** The file's time and size come from the operating system, which may
  cache them for seconds on a network share or a synced folder, so a change shows later there.
  A file that an editor rewrites to the same size within the file system's time resolution (2 s on
  FAT) is not seen until it changes again; a run always reads the file.
- **An unresponsive share can stall the daemon.** Looking at a file takes the daemon's lock, as the
  one-second check always did, so a registered file on a share that stops answering holds up the
  daemon until the share answers. A name that answers to nothing is looked up once more with a fresh
  look at every file, at most once per 250 ms, so asking for unknown names does not add to this. Looking
  at the files outside the lock is a follow-up.
- **A kept name does not survive a restart.** The name a file keeps while it does not load is held
  in memory. After the daemon restarts, a file that still does not parse is shown under its
  registered name until it loads again.
- **The same file by another spelling.** A file is the same file when its cleaned path is the same
  text. A symlink to it, or the same path in a different letter case on macOS or Windows, is another
  file to the registry, so registering it can create a second registration. Not new here; a
  follow-up with the case-insensitive names above.
- **A registration's reading is kept by its registered name.** If a name is unregistered and the same
  name registered again for another file of exactly the same size and time before the next check,
  the view could show the old reading until then. Runs always read the file. Not new here.
- **Names on case-insensitive volumes.** Registered names that differ only by case share a data
  folder on macOS and Windows. Not new here; a follow-up.

## Compatibility

Additive to API v1: optional fields `registered_name` on a pipeline and `registered_pipeline` on a run.
Two visible changes ride with them: `GET /pipelines` is sorted by the name shown, and registering a
file again without `--name` no longer makes a second registration. The registry file is unchanged and needs
no migration, except an optional `chosen` flag on an entry registered with `--name`; an older daemon
reads the file and ignores it. A registration made earlier with `--name` has no flag and follows the
file's name from now on; register it again with `--name` to keep a name. For a flow whose file does
not rename itself, nothing changes. For one that does, `name` in responses (and the `PIPELINE` column of `truss pipelines` and
`truss runs`) is now the file's name, and the registered name still works everywhere. Clients that
compare `name` with the name they registered should use `registered_name` or look the flow up by
either name. Runs add the optional `registered_pipeline` the same way.

## Verification

Go tests in `internal/daemon` (`follow_test.go`): a step edit reaches the view and the next run;
a rename keeps the history under the new name, answers to both names, makes no second registration
and drops the difference when the file names it back; a name another flow holds is not taken; a name
chosen with `--name` is kept.
A flow whose file breaks after a rename keeps its shown name and history; registering a file again
never duplicates it (a chosen name, a name two files claim); registering again with `--name` keeps
the name; a shown name is held (`409`) and works to unregister; runs name their registration and
tracked runs join the history; the log says once that a file changed. A browser test in
`tests/ui/pipelines.test.js` edits a flow's file, watches the page follow it, and re-runs an old
run by the registration.
Also tested: a file that could not be read heals without another edit (Unix; skipped on Windows and
as root); a file that fails on its content is not parsed again; the list order; registering a
file again when its new name is held by another flow; a run trigger that sees an edit first logs it;
`runs.ListFor` and `runs.SummaryFor` in `internal/runs`.
Not verified: Windows (nothing here is OS-specific; the file-time check is the one the daemon
already used); a file replaced by an editor that keeps the same size and time (the daemon would
not see that edit until its size or time changes; every run still reads the file); the response of
a cancelled run under a renamed flow (every run response passes the same mapping, tested for get,
list, trigger and tracked runs); a flow on a network share that stops answering.
