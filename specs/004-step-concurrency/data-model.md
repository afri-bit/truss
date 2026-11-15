# Data Model: Step Concurrency

Go names are working names for the plan. The external contracts (in [contracts/](./contracts/)) do not depend on them.

## Pipeline (`internal/pipeline`)

| Field | Type | Rule |
|---|---|---|
| `Concurrency` | integer | 0 means not declared, which means 1. A declared value is a decimal whole number of at least 1 (FR-001 to FR-003). |
| `unknownField` (private) | text | First unrecognised field in document order, with its location. Validate rejects it (FR-016). |
| `concurrencyBad`, `concurrencyText` (private) | flag, text | Set when the value is not a valid limit; Validate builds the error from the text. |

Recognised fields: top level `version`, `pipeline`, `steps`; under `pipeline` `name`, `concurrency`; per Step `id`, `run`, `needs`.

## ExecutionPlan

Adds `Concurrency` (at least 1). `Steps` keeps the spec 003 order; the scheduler scans the definition order itself and uses the plan to prove reachability.

## Step Run states (scheduler)

```text
waiting --(needs succeeded, slot free, no failure)--> running --> succeeded
                                                          \--> failed
                                                          \--> failed + stopped   (ended because another Step failed)
waiting --(failure recorded)--> never started (absent from the run)
```

## PipelineRun

| Field | Meaning |
|---|---|
| `Steps` | Started Steps in start order. Slot reserved at start, filled at finish. |
| `FailedStep` | Id of the first observed failure; empty on success. Drives the CLI error line. |
| `Status` | `succeeded` or `failed`. No new status. |

## StepRun

Adds `StoppedBy`: id of the Step whose failure caused this Step to be stopped; empty otherwise. A stopped Step has `Status` failed, its output so far, and either the Exit Code the host reported or a diagnostic and no Exit Code.

## executor.Result

| Field | Meaning |
|---|---|
| `ExitCode` | Set only when the host reports one. Nil for start failure and for a signal death. |
| `Err` | Start failure, or the signal diagnostic (`terminated by signal: killed`). |
| `Stopped` | TRUSS ended the process tree before the shell exited, and the exit was not a success. |

## Process tree (private to `internal/executor`)

One per started Step. Created before the shell runs, owns every descendant, and is released (and ended) when the Step's `Wait` returns or the context is cancelled. Unix: a process group. Windows: a job object.

## StepEvents (engine, consumer-owned)

Adds `StepStopped(id, failedID string)`, called after the Step's output and before `StepFinished`.

## Run log

One line type is added: `step <id> stopped because step <other> failed`. State shared by all Steps is guarded by one mutex; per-Step partial-line buffers are unchanged.
