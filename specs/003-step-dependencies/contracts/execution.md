# Contract: Sequential Execution with Dependencies

**Status**: Extends `specs/001-host-pipeline-engine/contracts/host-execution.md`  
**Compatibility**: Host shell, working directory, environment, and Exit Code rules are unchanged. Definition order remains the execution order when no Step has a non-empty `needs`. When `needs` is used, start order is the order below. Fail-fast now also skips Steps that do not need the failed Step. Concurrency, cancellation, timeouts, and retries stay undefined.

## Start order

Run one Step at a time. A Step may start only after every Step it needs has succeeded.

When more than one Step may start, start the one written earliest in the Pipeline Definition. Repeat until a Step fails or every Step has succeeded.

Omitted `needs` and an empty `needs` list are not Dependencies. File order is not a Dependency. The choice among Steps that may start is sequential execution for this feature, not an extra Dependency.

| Definition order | Start order |
|---|---|
| `test` needs `build`, then `build` | `build`, `test` |
| `package` needs `lint` and `test`, then `lint`, then `test` | `lint`, `test`, `package` |
| `report` needs `build`, then `build`, then `scan` | `build`, `report`, `scan` |
| `publish` needs `package`, then `lint`, then `package` with no Dependencies | `lint`, `package`, `publish` |
| No Step uses `needs` | Definition order |

Each started Step is still one new host-shell process. The command form, working directory, and environment are the host-execution contract.

## Failure

| Process outcome | Step status | Pipeline status | Further Steps |
|---|---|---|---|
| Exit Code 0 | `succeeded` | continues | the next Step in start order |
| Exit Code other than 0 | `failed` | `failed` | do not start |
| Process cannot start | `failed`, with a diagnostic and no Exit Code | `failed` | do not start |

Further Steps include Steps that need the failed Step and Steps that were already allowed to start. They do not appear as `succeeded`.

The Exit Code stored on a started process is the process Exit Code.

## Out of scope

Concurrent Steps, cancellation status, timeouts, retries, conditional Steps, container execution, and remote execution.
