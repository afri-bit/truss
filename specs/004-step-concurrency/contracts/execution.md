# Contract: Concurrent Execution

**Status**: Extends `specs/003-step-dependencies/contracts/execution.md` and `specs/001-host-pipeline-engine/contracts/host-execution.md`  
**Compatibility**: With the limit omitted or 1, start order, fail-fast, and Exit Code rules are exactly spec 003, with three deliberate changes: leftover processes are ended when a Step's shell exits; a Step ended by a signal shows a diagnostic instead of `-1`; unknown fields are rejected (see pipeline-v1.md). No version has been tagged.

## Start rule

At any moment, TRUSS may start a Step when: it has not started; every Step it needs has succeeded; fewer Steps than the limit are running; and no Step has failed. When several Steps may start, the ones written earliest in the Pipeline Definition start first. TRUSS repeats this each time a running Step finishes.

A Step's position in the file is not a Dependency.

| Limit | Definition | Start behavior |
|---|---|---|
| 2 | `a`, `b`, `c`, no Dependencies | `a` and `b` together; `c` after one of them finishes |
| 3 | `build`, `lint`, `test` needs `build` | `build` and `lint` together; `test` after `build`, not waiting for `lint` |
| 2 | `a`, `b` needs `a`, `c`, `d` needs `c` | `a` and `c` together; `b` after `a`; `d` after `c` |
| 1 or omitted | any | spec 003 order |

Each started Step is still one new host-shell process, with the working directory and environment of the host-execution contract.

## Process tree

Each Step's shell is the root of a tree that TRUSS can end as a unit: a process group on Linux and macOS; a job object on Windows. A process the Step's command starts belongs to that tree.

| Event | Required result |
|---|---|
| A Step's shell exits (any outcome) | Every process of that Step still running is ended before the Step is reported finished |
| A Step fails | No further Step starts; every running Step's whole tree is ended immediately and forcefully, with no polite request and no wait |
| `truss` is interrupted (Ctrl-C, SIGTERM) | Same as a failure, then `truss` exits non-zero (see cli.md) |
| `truss` exits | Nothing a Step started is still running |

Limitation: a process that leaves the tree on its own (for example `setsid` on Unix) can survive.

## Outcome of a Step

| Process outcome | Step status | Log shows |
|---|---|---|
| Exit Code 0 | `succeeded` | `exit 0` |
| Exit Code other than 0 | `failed` | `exit <code>` |
| Cannot be started | `failed` | diagnostic, no Exit Code |
| Ended by a signal (any cause) | `failed` | diagnostic, no Exit Code; never `-1` |
| Stopped by TRUSS | `failed` | the `stopped` line, then the Exit Code the host reports (Windows: `exit 1`) or the signal diagnostic (Unix) |

A Step that exits 0 on its own while a stop is in progress is `succeeded`.

## Failure

The Pipeline Run is `failed`. The run's error names the pipeline and the first failure TRUSS observed. Every started Step's own Exit Code or diagnostic stays in the log. Steps that did not start are absent and not shown as succeeded.

## Out of scope

Cancellation as a reported status, timeouts, retries, conditional Steps, a polite shutdown, container execution, and remote execution.
