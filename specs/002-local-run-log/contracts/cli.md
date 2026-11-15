# Contract: CLI

**Status**: Replaces the invocation and transcript rules in `specs/001-host-pipeline-engine/contracts/cli.md`  
**Compatibility**: Replaces the unreleased host-engine transcript. Schema version `"1"` is unchanged. `truss` with no command, `truss help`, and `truss --help` exit 0. `truss version` and `truss --version` are new. `truss run [<path>]`, fail-fast, and the process exit for a succeeded or failed run stay. The run log contract is [run-log.md](./run-log.md). *Changed by [spec 016](../../016-project-layout/spec.md): Bare `truss run` exits 1 and lists the flows. The saved log is `.truss/state/<flow>/run.log`.*

## Commands

| Invocation | Result | Exit |
|---|---|---|
| `truss`, `truss help`, `truss --help` | Usage on standard output. No step. No run log. | 0 |
| `truss version`, `truss --version` | One line, the product version, on standard output. No step. No run log. | 0 |
| `truss run` | Load `truss.yaml` from the working directory and run it. *Changed by [spec 016](../../016-project-layout/spec.md): Now exits 1 and lists the flows in `.truss/flows/`.* | 0 or 1 |
| `truss run <path>` | Load that path and run it. `truss run --help` is a path. | 0 or 1 |
| Anything else | Usage on standard error. No step. No run log. | 1 |

Usage text:

```text
usage: truss run [<path>]
       truss help
       truss version
```

*Changed by [spec 016](../../016-project-layout/spec.md): The usage is `truss run <name|path>`. A bare `truss run` exits 1 and lists the flows.*

The product version is the `// version:` value declared for the module. The version command prints that value and no other text.

## Process exit for `truss run`

| Outcome | Exit |
|---|---|
| Pipeline run succeeded and `.truss/run.log` was replaced | 0 |
| Definition missing, unreadable, invalid YAML, or invalid | 1 |
| Working directory unknown, or `.truss/run.log` cannot be prepared | 1 |
| Pipeline run failed | 1 |
| The finished log cannot replace `.truss/run.log` | 1 |

*Changed by [spec 016](../../016-project-layout/spec.md): The log path in the rows above is `.truss/state/<flow>/run.log`.*

The process exit is not the step Exit Code. The step Exit Code stays in the run log.

## Run log

A run that starts writes the [run log](./run-log.md) to standard output as it proceeds. The same bytes replace `.truss/run.log` after the run finishes. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*

Load, parse, and validation failures print this on standard error and do not write a run log:

```text
error: <diagnostic naming the problem>
```

An existing `.truss/run.log` stays unchanged in that case. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*

An unwritable `.truss` directory or temporary log prints a user-visible error on standard error, starts no step, and leaves an existing `.truss/run.log` unchanged. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*
