# Research: Step Dependencies

## Where the graph lives

- **Decision**: Add `Needs []string` to `pipeline.Step`. Parse it in `Parse`. Reject illegal graphs in `Validate`. Put the sequential order in `Plan` by reordering `ExecutionPlan.Steps`. Leave `engine.Run` as a loop over that slice that returns on the first failed Step.
- **Rationale**: The spec keeps execution sequential and forbids recording the tie-break as a Dependency. The plan slice is the order for this feature. `Needs` remains the declared edges. The engine already stops the run on failure, which covers both dependents and ready Steps that have not started. The constitution forbids a package that exists only for a later concurrent scheduler.
- **Alternatives considered**: A new `internal/graph` package (rejected: one validation pass and one ordering pass do not justify a package). Teaching the engine to pick the next ready Step itself (rejected: the same rule would live in two places, and the plan would no longer be the execution order). Dropping `Needs` after planning (rejected: the declared edges would survive only as order, which is the tie-break the spec says is not a Dependency).

## Scheduling rule

- **Decision**: After validation, `Plan` repeatedly appends the not-yet-chosen Step with the smallest definition index whose every `Needs` id is already chosen. Omitted `needs` and `needs: []` are both an empty list, so those Steps are eligible immediately. If a Step remains and none is eligible, `Plan` returns an error. `Validate` already rejects that case.
- **Rationale**: That selection is FR-006. Scanning definition order needs no heap and no extra dependency. An empty graph selects index 0, then 1, then 2, which is today's definition order. The four orders in User Story 1 fall out of the same rule: `build` then `test`; `lint` then `test` then `package`; `build` then `report` then `scan`; `lint` then `package` then `publish`.
- **Alternatives considered**: Sort with a library heap (rejected: the standard library is enough, and pipelines here are one local file). Run every eligible Step in a goroutine (rejected: concurrency is out of scope). Treat file order as an implicit `needs` edge (rejected by FR-002).

## YAML shape

- **Decision**: Keep the existing `yaml.v3` node walk. On a Step, `needs` may be absent or a sequence. Each sequence entry must be a string scalar (`!!str`), trimmed the same way a Step `id` is trimmed. Any other shape sets the same flag the validator uses for "not a list of step ids". Other keys, including `depends_on`, stay ignored. `needs` outside a Step is ignored.
- **Rationale**: Step ids are already trimmed at parse, so a `needs` entry must be trimmed or `" build "` would not match `build`. Requiring `!!str` matches `run` and schema version `"1"`: a number is not a Step id. Ignoring unknown fields is the v1 contract this feature does not widen, and the spec says `depends_on` is not the field.
- **Alternatives considered**: Accept a single string as a one-element list (rejected: FR-004 rejects a `needs` value that is not a list). Reject unknown fields now (rejected: that would change definitions the spec says stay valid). Decode into a map and let yaml.v3 coerce numbers (rejected: the current parser keeps string-versus-number checks explicit).

## Validation order and diagnostics

- **Decision**: Existing structural checks stay first and keep their current errors. Then, for each Step in definition order, check `needs`. A field that is not a list of string Step ids produces `step %q needs must be a list of step ids`. Otherwise each entry is checked in list order and the first fault wins: unknown id, including an empty entry, is `step %q needs unknown step %q`; the Step's own id is `step %q depends on itself`; a repeated id is `step %q repeats step %q`. After every list is clean, one cycle check produces `dependency cycle: <ids>`.
- **Rationale**: Story 4 requires today's errors when `needs` is absent or when an older problem is present. Story 2 requires the offending Step, and the unknown or repeated id, to appear in the error. One first-fault order makes the contract tests deterministic when a definition has more than one problem. The CLI already prints `error: <diagnostic>` and writes no run log.
- **Alternatives considered**: Collect every problem (rejected: the spec does not require a list, and the current validator returns one error). One shared "invalid needs" sentence (rejected: a cycle, a self-dependency, and an unknown id are different acceptance scenarios).

## Which cycle is named

- **Decision**: Search in definition order, and explore each `needs` list in list order. The edge direction is "this Step needs that Step". The first time the search returns to a Step already on the current path, that path segment is the cycle. Rotate it so the first name is the member written earliest in the file, then follow the `needs` edges. Name each member once, separated by `, `. Example: `a` needs `b`, `b` needs `c`, `c` needs `a`, file order `a`, `b`, `c` → `dependency cycle: a, b, c`.
- **Rationale**: FR-004 requires every Step in the cycle to be named. A fixed search and a fixed rotation make that list stable. A self-dependency is reported earlier, so a Step that needs itself is not described only as a cycle.
- **Alternatives considered**: Name Steps in definition order without following the cycle (rejected: the list would be stable but would not show the cycle). Report every cycle in the file (rejected: one cycle is enough to reject the definition).

## Run log and CLI

- **Decision**: Do not change `internal/runlog` or the CLI command set. Step lines appear in plan order because that is the order the engine emits. The log gains no `needs` line. `.truss/run.log`, help, version, and process exit codes stay as in the local run log.
- **Rationale**: FR-009 and FR-011 keep the current format and load behavior. Order is an execution fact, not a new log field. Presentation stays out of the engine.
- **Alternatives considered**: Print `needs` on the `started` line (rejected by the spec). Reorder the log in the CLI after the run (rejected: the live log must already be in run order).

## What stays

- **Decision**: Schema version `"1"`. Host shell, working directory, environment, Exit Code preservation, and fail-fast stay. Cancellation is still not a specified behavior; the existing `context.Context` checks in the engine stay and this feature adds no cancellation contract. No secret store and no environment dump are added.
- **Rationale**: The spec retains version `"1"` because `needs` is optional and no release is tagged. Definitions without `needs` must match the host pipeline engine and the local run log. Concurrency, containers, and remote workers stay out of scope.
- **Alternatives considered**: Bump the schema to `"2"` when `needs` is present (rejected: the spec keeps `"1"`, and a bump would reject current files that do not use the field).
