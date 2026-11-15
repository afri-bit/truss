# Contract: `truss run`

**Status**: New public contract  
**Compatibility**: First CLI contract. The CLI streams a log and does not own execution behavior. This transcript shape is not yet released.

## Usage

```text
truss run [<path>]
```

*Changed by [spec 016](../../016-project-layout/spec.md): Bare `truss run` is an error that lists the flows in `.truss/flows/` (exit 1). Use `truss run <name>` or `truss run <path>`.*

- With `<path>`: load that path as given. Any filename is valid.
- Without `<path>`: load `truss.yaml` from the invoker's working directory. *Changed by [spec 016](../../016-project-layout/spec.md): Bare `truss run` exits 1 and lists the flows in `.truss/flows/`.*
- Any other invocation is a usage error, exits 1, and executes nothing.

## Process Exit Code

| Outcome | Exit code |
|---|---|
| Pipeline Run `succeeded` | 0 |
| Missing or unreadable file, invalid YAML, validation error, or Pipeline Run `failed` | 1 |

The process Exit Code is not the Step Exit Code. On failure, the Step Exit Code appears in the error line.

## Success transcript

Printed to standard output as each Step runs. No pipeline status block and no per-step exit code.

```text
[<id>]
<stdout and stderr as produced>
```

Repeat for every started Step, in definition order. Empty output adds no extra lines.

## Failed Step transcript

The same log, then one line on standard error. Later Steps are omitted.

```text
error: <pipeline name> failed at <id> (exit <n>)
```

When the process did not start:

```text
error: <pipeline name> failed at <id>: <diagnostic>
```

## Load and validation errors

Printed to standard error. No `step` block. Process exits 1.

```text
error: <diagnostic naming the problem>
```
