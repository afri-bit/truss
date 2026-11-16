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
  edit by up to a second. A file that no longer loads is shown as invalid with the reason, and its
  history stays.
- **FR-002** Each registration has a **registered name**: the name it was registered under. It never
  changes. Run history, schedules, overlap control and the data and cache folders hang on it.
- **FR-003** The name the daemon **shows** for a registration is `pipeline.name` from its file, when
  that is a valid registration name and no other registration holds or claims it. Otherwise it is the
  registered name. A name chosen at registration (`--name`, or the name field in Desktop) is never
  replaced by the file's. One name never answers for two registrations: a registered name always wins, and
  when two files claim the same new name, both keep their registered names.
- **FR-004** `name` in the API's pipeline object is the shown name. `registered_name` is present only
  when it differs. `definition_name` is unchanged.
- **FR-005** Every command and API call that takes a pipeline name accepts the registered name or the
  shown name: `GET`, `DELETE` and the `pause`, `resume` and `runs` calls of `/pipelines/{name}`, and
  `GET /runs?pipeline=`. A name that answers to no registration is an error as before (`404`).
- **FR-006** The history of a flow is every run of its registered name, plus any run recorded under
  its shown name (a run tracked from a shell reports the file's name). `GET /runs?pipeline=` and the
  pipeline's `last_run` and `active_runs` use it.
- **FR-007** A run's `pipeline` is the shown name of its registration, in every response. A run
  record on disk keeps the registered name; nothing in the run history is rewritten.
- **FR-008** Registering a file that a registration already answers to, by the name its file gives
  (as `truss register --all` does), returns that registration and creates no second one. Registering
  another file under a name a registration holds or shows is `409`, as before.
- **FR-009** The daemon log has a line when a registered file changed and was read again, with the
  reason when it no longer loads and the file's name when it differs from the registered name.
- **FR-010** TRUSS Desktop shows the registered name on the flow's page, as *Registered as*, when it
  differs. Lists, the run page and *Jump to…* use the shown name, and the page follows an edit by
  itself.

## Not in this spec

Changing a registration's registered name (to do so, unregister and register again; the history stays
with the old name); history of more than the registered name and the shown name (a flow renamed
twice finds only runs tracked under its current name and runs started by the daemon); watching with
file-system events (no new dependency; one second is enough); moving data or cache folders;
`TRUSS_PIPELINE` in a step's environment, which stays the registered name so a run's environment
does not change under a flow.

## Compatibility

Additive to API v1: one optional field, `registered_name`. The registry file is unchanged and needs
no migration, except an optional `chosen` flag on an entry registered with `--name`; an older daemon
reads the file and ignores it. A registration made earlier with `--name` has no flag and follows the
file's name from now on; register it again with `--name` to keep a name. For a flow whose file does
not rename itself, nothing changes. For one that does, `name` in responses (and the `PIPELINE` column of `truss pipelines` and
`truss runs`) is now the file's name, and the registered name still works everywhere. Clients that
compare `name` with the name they registered should use `registered_name` or look the flow up by
either name.

## Verification

Go tests in `internal/daemon` (`follow_test.go`): a step edit reaches the view and the next run;
a rename keeps the history under the new name, answers to both names, makes no second registration
and drops the difference when the file names it back; a name another flow holds is not taken; a name
chosen with `--name` is kept.
A browser test in `tests/ui/pipelines.test.js` edits a flow's file and watches the page follow it.
Not verified: Windows (nothing here is OS-specific; the file-time check is the one the daemon
already used) and a file replaced by an editor that keeps the same size and time (the daemon would
not see that edit until its size or time changes; every run still reads the file).
