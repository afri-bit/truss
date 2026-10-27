# Data Model: Step Dependencies

Schema version stays `"1"`. This feature adds one optional field on a Step. The run log model from the local run log feature stays; started Steps are ordered by execution, not by a new log field.

## Pipeline Definition

| Field | Required | Rules |
|---|---|---|
| `version` | Yes | The string `"1"`. Unchanged. |
| `pipeline.name` | Yes | Non-empty string. Unchanged. |
| `steps` | Yes | A non-empty list. Unchanged. |
| `steps[].id` | Yes | Non-empty string, unique in the file. Trimmed. Compared exactly, including case. |
| `steps[].run` | Yes | Non-empty string. Unchanged. |
| `steps[].needs` | No | A list of Step ids. Omitted and `[]` both mean no Dependencies. Each entry is a string, trimmed the same way as `id`. |

Unknown fields stay ignored. `depends_on` does not declare a Dependency.

## Step

| Field | Required | Rules |
|---|---|---|
| ID | Yes | The trimmed `id`. |
| Run | Yes | The command string. |
| Needs | No | The declared Dependency targets, in list order. Empty when `needs` is omitted or empty. The list order does not change execution order. Duplicates are invalid. |

A Step's position in the file is not a Dependency.

## Validation

Checked before any Step command starts. Existing structural errors keep their current text and are reported before a `needs` error. For `needs`, the first fault in definition order wins, and inside one list the first entry fault wins.

| Fault | Error names |
|---|---|
| `needs` is missing | Not a fault. |
| `needs` is `[]` | Not a fault. |
| `needs` is not a list of string Step ids | `step "<id>" needs must be a list of step ids` |
| Entry is not the id of a Step in this file, including `""` | `step "<id>" needs unknown step "<entry>"` |
| Entry is this Step's id | `step "<id>" depends on itself` |
| Entry repeats an earlier entry in the same list | `step "<id>" repeats step "<entry>"` |
| The lists contain a cycle | `dependency cycle: <id>, <id>, ...` |

The cycle list starts at the member written earliest in the file and then follows `needs`. Each member appears once. A self-dependency is reported as the self fault, not as a cycle.

A rejected definition writes no run log and leaves an existing `.truss/run.log` unchanged.

## Execution Plan

The ordered list of Steps for one validated Pipeline. Slice order is the order Steps start. `Needs` on each Step is still only the declared list.

A Step is eligible when it has not been chosen and every id in `Needs` has already been chosen. The plan repeatedly chooses the eligible Step with the smallest definition index until every Step is chosen.

| Definition | Plan order |
|---|---|
| No Step has a non-empty `needs` | Definition order |
| `test` needs `build`, then `build` | `build`, `test` |
| `package` needs `lint` and `test`, then `lint`, then `test` | `lint`, `test`, `package` |
| `report` needs `build`, then `build`, then `scan` | `build`, `report`, `scan` |
| `publish` needs `package`, then `lint` with `needs` omitted, then `package` with `needs: []` | `lint`, `package`, `publish` |

The choice among eligible Steps is not stored as a Dependency.

## Pipeline Run

Unchanged. `succeeded` or `failed`. `Steps` contains only Steps that started, in the order they started. A failed Step ends the run. Later plan Steps, including Steps that were already eligible and do not need the failed Step, are absent and are not `succeeded`.

## Run Log

Unchanged in form from the local run log. Started Steps appear in Pipeline Run order. No line lists `needs`. The summary counts Steps that started.

## Relationships

- A Pipeline Definition has one or more Steps.
- A Step has zero or more Dependencies, each naming another Step.
- An Execution Plan has the same Steps as the validated definition, in start order.
- A Pipeline Run has the prefix of that plan that started.
- A Run Log describes that Pipeline Run and does not copy the Execution Graph.

## State

| Situation | Steps started | Latest run log |
|---|---|---|
| Definition cannot be read, parsed, or validated, including an illegal `needs` graph | None | Unchanged |
| Every Step succeeds | All, in plan order | Replaced |
| A Step fails or cannot start | The plan prefix through that Step | Replaced with that failed run |
