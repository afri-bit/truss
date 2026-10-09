# Contract: Sequential Host Execution

**Status**: New public contract  
**Compatibility**: Fail-fast, definition-order host execution is a versioned Execution Semantics contract.

## Invocation

For each Step, in definition order, start a new host-shell process:

| Host | Command |
|---|---|
| Unix-like | `/bin/sh -c <run>` |
| Windows | `%COMSPEC% /c <run>`, or `cmd.exe /c <run>` when `COMSPEC` is unset |

`<run>` is the Step string unchanged. It is not split into an argument list by TRUSS. The shell interprets quotes, pipes, and environment expansion.

## Process context

- Working directory: the invoker's working directory, captured once at the start of the run.
- Environment: the invoker's environment, captured once at the start of the run.
- Each Step is a new process. Earlier Steps do not change the directory or environment of later Steps.
- Stdin is empty. Stdout and stderr are captured and are not an environment listing added by TRUSS.

## Results

| Process outcome | Step status | Pipeline status | Later Steps |
|---|---|---|---|
| Exit Code 0 | `succeeded` | continues | run |
| Exit Code other than 0 | `failed` | `failed` | do not run |
| Process cannot start | `failed`, with a diagnostic and no Exit Code | `failed` | do not run |

The Exit Code stored on a started process is the process Exit Code. It is not replaced, ignored, or mapped to 0.

## Out of scope

Concurrent Steps, dependency edges, cancellation status, timeouts, retries, skip conditions, container execution, and remote execution.
