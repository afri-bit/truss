# 0017. A registration keeps its name; the file names it

- Status: Proposed
- Date: 2026-11-16

## Context

A flow was registered under a name, and the file also has `pipeline.name`. After the maintainer
renamed a flow in its file, Desktop and the CLI kept the old name while the steps changed: two
names for one thing, and no clear owner. The maintainer decided on 2026-11-16 that the file is the
source of truth, that the daemon should notice a change on its own, and that history must stay.

## Decision

A registration has an immutable **registered name** (what history, schedules, overlap control and
the data and cache folders hang on) and a **shown name** that follows `pipeline.name`. The daemon
shows the file's name unless it is not a valid registration name or another registration holds or
claims it, or the name was chosen with `--name`; then the registered name stays. Every command accepts either name, and a run reports
the shown name while its record keeps the registered one. History is therefore never rewritten or
moved. Watching stays a one-second check of size and modification time, plus a check on every
request that shows the file. File-system events were not chosen: they add a third-party module
and behave differently per OS.

## Consequences

- No registry migration and no rewritten run records; an older daemon still reads the registry. The
  one new field, `chosen`, is optional. A registration made with `--name` before this change has
  none and follows the file; the maintainer's release is still a beta.
- Two names can answer for one flow. The API says so (`registered_name`) and Desktop shows it.
- A flow renamed twice keeps the history of its registered name and of its current name only.
- A shown name can move to another flow when claims change. Scripts and Desktop therefore act on the
  registered name, and runs carry it (`registered_pipeline`). A file that stops loading keeps the
  name it was last read with, so a name does not move while a file is being fixed.
- A file registered again without `--name` is never registered twice, and is never a name clash;
  `--name` is the one way to give a file a second registration. The path is compared as text, so a
  symlink or a letter-case variant of the same file is another file to the registry (a follow-up).
- A file that fails on its content is read once per change, and one that could not be read is read
  again at every check, so a daemon is not kept busy by a broken file.
- Following relies on the file's time and size, which a network share may report late (spec 019,
  known limits).
- Name resolution lives in `internal/daemon`, because it needs the file's content; the registry
  stays a store of names and paths.
- `TRUSS_PIPELINE` in step environments stays the registered name, so a run's environment does not
  change when a file renames itself.
