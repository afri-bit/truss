# Data Model: Host-Executed Pipeline Engine

## Pipeline Definition (YAML v1)

External document. Not an execution record.

| Field | Required | Rule |
|---|---|---|
| `version` | yes | String `"1"` only |
| `pipeline.name` | yes | Non-empty string after trimming whitespace |
| `steps` | yes | Non-empty list |
| `steps[].id` | yes | Non-empty string after trimming whitespace; unique in the list; comparison is case-sensitive |
| `steps[].run` | yes | Non-empty string; executed as one host-shell string |

Unknown fields are ignored. `env`, working directory, `depends_on`, image, cache, matrix, and artifacts are not part of this model.

## Pipeline Model

Internal representation after a successful parse, before validation may still reject it.

- `Version` (string)
- `Name` (string)
- `Steps` (ordered list of Step)

A Pipeline Model is not a Pipeline Run. Parsing it executes nothing.

## Step

- `ID` (string)
- `Run` (string)

Order is the list order from the definition.

## Execution Plan

Ordered list of Steps taken from a validated Pipeline Model. The order is definition order. There is no dependency graph and no parallel group.

## Pipeline Run

One execution of one Execution Plan.

| Field | Rule |
|---|---|
| `Name` | Pipeline name |
| `Status` | `succeeded` if every executed Step succeeded; otherwise `failed` |
| `Steps` | Only Steps that were started, in plan order |
| `Error` | Set for load, parse, or validation failure, and when a Step cannot start |

Steps that were never started are absent. They are not `skipped` or `succeeded`.

## Step Run

One started Step.

| Field | Rule |
|---|---|
| `ID` | Step id |
| `Status` | `succeeded` or `failed` |
| `ExitCode` | Process Exit Code when the process started. Absent when the process could not start. `0` succeeds; any other value fails |
| `Output` | Stdout text. Empty is valid |
| `Stderr` | Stderr text. Shown to the user. Not an environment dump |
| `Error` | Diagnostic when the process could not start or the run could not continue |

## Status transitions

```text
validated plan
    → start Step
        → process exit 0 → Step succeeded
        → process exit non-zero → Step failed → Pipeline failed → stop
        → process did not start → Step failed → Pipeline failed → stop
    → no Steps left → Pipeline succeeded
```

Parse and validation failures never create Step Runs.

## Relationships

- A Pipeline Definition parses into one Pipeline Model.
- A valid Pipeline Model produces one Execution Plan.
- One Execution Plan produces one Pipeline Run per invocation.
- A Pipeline Run contains zero or more Step Runs.
- The Host Executor creates a Step Run from one Step. It does not read the CLI.
