# Contract: `truss run`

**Status**: New public contract  
**Compatibility**: First CLI contract. The CLI prints a human-readable Result and does not own execution behavior.

## Usage

```text
truss run [<path>]
```

- With `<path>`: load that path as given. Any filename is valid.
- Without `<path>`: load `truss.yaml` from the invoker's working directory.
- Any other invocation is a usage error, exits 1, and executes nothing.

## Process Exit Code

| Outcome | Exit code |
|---|---|
| Pipeline Run `succeeded` | 0 |
| Missing or unreadable file, invalid YAML, validation error, or Pipeline Run `failed` | 1 |

The process Exit Code is not the Step Exit Code. The Step Exit Code appears in the report.

## Success report

Printed to standard output:

```text
pipeline: <name>
status: succeeded

step: <id>
status: succeeded
exit_code: <n>
output:
<stdout, or an empty line when stdout is empty>
stderr:
<stderr, or an empty line when stderr is empty>
```

Repeat the `step` block in execution order for every started Step.

## Failed Step report

Same header with `status: failed`. The failed Step includes its `exit_code` when the process started. Later Steps are omitted.

```text
pipeline: <name>
status: failed

step: <id>
status: failed
exit_code: <n>
output:
<stdout>
stderr:
<stderr>
```

When the process did not start, omit `exit_code` and include:

```text
error: <diagnostic>
```

## Load and validation errors

Printed to standard error. No `step` block. Process exits 1.

```text
error: <diagnostic naming the problem>
```
