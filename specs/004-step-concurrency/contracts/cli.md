# Contract: CLI

**Status**: Extends `specs/002-local-run-log/contracts/cli.md`  
**Compatibility**: Usage, load behavior, and exit codes are unchanged except for interrupt handling, which was undefined. No version has been tagged.

## Interrupt

If `truss run` receives SIGINT (Ctrl-C) or SIGTERM while a Pipeline Run is in progress:

- no further Step starts;
- every running Step's whole process tree is ended immediately and forcefully;
- `truss` prints `error: context canceled` on standard error and exits 1;
- the temporary log is discarded and an existing `.truss/run.log` is unchanged; *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*
- no `cancelled` status is reported.

## Exit codes

Unchanged: 0 only when the Pipeline Run succeeded and the latest run log was saved; non-zero when the definition was rejected, the log location was unusable, the run failed, or the run was interrupted.

## Failure line

For a failed run, the summary's `error` line names the pipeline and the first failure TRUSS observed (see execution.md).
