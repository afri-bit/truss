# Feature Specification: Step Dependencies

**Feature Branch**: `003-step-dependencies`  
**Created**: 2026-10-27  
**Status**: Draft  
**Input**: User description: "Add dependency graphs between Steps to the Pipeline Definition. A Step may declare the Steps that must succeed before it may start. Execution stays sequential and deterministic. Invalid graphs are rejected before any command starts. Definitions that declare no dependencies keep today's behavior."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run steps after the steps they need (Priority: P1)

A developer names, on a Step, the other Steps that must succeed before that Step may start. TRUSS runs the pipeline one Step at a time in an order that honors every declared Dependency. When more than one Step is ready, the Step written earlier in the Pipeline Definition runs first. A Step that declares no Dependencies is not treated as waiting on the Step written above it.

**Why this priority**: Declared order is the behavior this feature exists to provide. Validation, failure handling, and compatibility matter only once a valid graph can run.

**Independent Test**: Run a valid Pipeline Definition whose first Step needs a Step written later in the file, and whose commands all succeed. Confirm the needed Step runs first, the dependent Step runs only after it, and both appear in that order in the run log with timestamps, durations, and Exit Codes.

**Acceptance Scenarios**:

1. **Given** a valid Pipeline Definition whose earlier Step `test` needs a later Step `build`, and both commands succeed, **When** the developer runs it, **Then** `build` runs and succeeds before `test` starts, and the run log shows `build` then `test`.
2. **Given** definition order `package` (needs `lint` and `test`), then `lint`, then `test`, and all commands succeed, **When** the developer runs it, **Then** the run order is `lint`, then `test`, then `package`. `package` starts only after both `lint` and `test` have succeeded.
3. **Given** definition order `report` (needs `build`), then `build`, then `scan`, and no other Dependencies, and all commands succeed, **When** the developer runs it, **Then** the run order is `build`, then `report`, then `scan`.
4. **Given** definition order `publish` (needs `package`), then `lint` with `needs` omitted, then `package` with an empty `needs` list, and all commands succeed, **When** the developer runs it, **Then** the run order is `lint`, then `package`, then `publish`. `lint` and `package` have no Dependencies, so `lint` starts before `publish` even though `publish` is written first.
5. **Given** a successful Pipeline Run that used Dependencies, **When** the developer reads the run log, **Then** each started Step shows its id, start time, output, finish time, duration, and Exit Code, the setup section still shows the environment, operating system, architecture, host shell, and working directory, the summary still reports status, the number of Steps that started, the run's duration, and the finish time, and the log does not list the Dependencies of any Step.

---

### User Story 2 - Reject an invalid dependency graph (Priority: P2)

A developer writes a Dependency that cannot be run: an unknown Step id, a Step that needs itself, a cycle, a repeated id in one list, or a `needs` value that is not a list of Step ids. TRUSS explains the problem, names the offending Step, starts no command, and leaves any existing latest run log unchanged.

**Why this priority**: Running an impossible graph would start commands in an order the developer did not ask for. Rejection has to be trustworthy on its own.

**Independent Test**: For each invalid case, run that definition while a latest run log already exists. Confirm an error names the offending Step, no Step command runs, and the stored log is byte-for-byte unchanged.

**Acceptance Scenarios**:

1. **Given** a Step whose `needs` list names a Step id that is not in the Pipeline Definition, **When** the developer runs it, **Then** TRUSS reports a validation error that names that Step and the unknown id, starts no Step, writes no new run log, and leaves an existing latest run log unchanged.
2. **Given** a Step whose `needs` list includes its own id, **When** the developer runs it, **Then** TRUSS reports a validation error that names that Step and states that it depends on itself, and starts no Step.
3. **Given** two or more Steps whose Dependencies form a cycle, **When** the developer runs it, **Then** TRUSS reports a validation error that names every Step in the cycle, and starts no Step.
4. **Given** one Step whose `needs` list repeats the same Step id, **When** the developer runs it, **Then** TRUSS reports a validation error that names that Step and the repeated id, and starts no Step.
5. **Given** a Step whose `needs` value is present and is not a list of Step ids, **When** the developer runs it, **Then** TRUSS reports a validation error that names that Step, and starts no Step.
6. **Given** any of the definitions in scenarios 1 through 5, **When** the developer runs it, **Then** the same rejection rules as the local run log apply: no command starts, and an existing `.truss/run.log` is unchanged.

---

### User Story 3 - Stop the whole run when a step fails (Priority: P3)

A developer runs a pipeline whose Steps name Dependencies. One started Step fails. TRUSS fails the Pipeline Run, starts nothing further, and saves a run log that contains only the Steps that started. Steps that were ready, and Steps that were still waiting, are omitted and are not shown as succeeded.

**Why this priority**: Fail-fast is already the execution contract. Dependencies must not weaken it by letting an unrelated Step continue after a failure.

**Independent Test**: Run a definition whose first ready Step exits non-zero and whose other Steps would print distinctive output, including one Step that needs the failed Step and one Step that needs nothing. Confirm only the failed Step ran, its Exit Code is in the saved log, and the other Steps' output is absent.

**Acceptance Scenarios**:

1. **Given** definition order `build`, then `test` (needs `build`), then `lint`, and `build` exits non-zero, **When** the developer runs it, **Then** only `build` starts, the Pipeline Run fails, `test` and `lint` do not start, and neither is shown as succeeded.
2. **Given** definition order `lint`, then `test`, then `package` (needs `lint` and `test`), `lint` succeeds, and `test` exits non-zero, **When** the developer runs it, **Then** the run log contains `lint` and the failed `test` with its output, start time, finish time, duration, and Exit Code, and omits `package`.
3. **Given** a Step that is selected to run and whose command cannot be started, **When** the developer runs it, **Then** that Step is shown as failed with a start time, a finish time, a duration, a diagnostic, and no Exit Code, no further Step starts, and the log is saved.
4. **Given** a failed Pipeline Run that used Dependencies, **When** the developer opens the latest run log, **Then** the saved text is the log just shown, the summary reports failure, and the error names the pipeline and the failed Step.

---

### User Story 4 - Leave definitions without dependencies unchanged (Priority: P4)

A developer runs a Pipeline Definition that does not use `needs`. Steps still run in definition order, the first failed Step still fails the Pipeline Run, and the run log, load behavior, and process exit codes stay as they are today.

**Why this priority**: Existing pipelines are the regression baseline. The new field is optional, so unused pipelines must not change.

**Independent Test**: Run the same successful definition and the same failed three-Step definition accepted by the local run log feature, neither of which contains `needs`. Confirm definition order, fail-fast omission of later Steps, saved log text, and process exit codes match that feature.

**Acceptance Scenarios**:

1. **Given** a valid Pipeline Definition that does not contain `needs` and whose commands succeed, **When** the developer runs it, **Then** Steps run in definition order and the run log matches the local run log contract for a successful run.
2. **Given** a valid Pipeline Definition that does not contain `needs` and whose second Step exits non-zero, **When** the developer runs it, **Then** the first Step and the failed second Step appear, later Steps are omitted, the log is saved, and the process exits non-zero.
3. **Given** a definition that is already invalid for a missing name, an empty Step list, a duplicate Step id, an empty `run`, or an unsupported schema version, **When** the developer runs it, **Then** TRUSS reports the existing validation error, starts no Step, and leaves an existing latest run log unchanged.
4. **Given** `truss run` with no path, **When** the invoker's working directory contains `truss.yaml`, **Then** TRUSS loads that file. **Given** `truss run` with a path, **When** the developer runs it, **Then** TRUSS loads that file. The process exits 0 only when the Pipeline Run succeeded and the latest run log was saved.

---

### Edge Cases

- `needs` is omitted. The Step has no Dependencies.
- `needs` is an empty list. The Step has no Dependencies.
- A Step needs one Step, or several Steps. Every listed Step must have succeeded before it starts.
- The same Step id is needed by more than one Step.
- A Step needs another Step that is written later in the file.
- The order of ids inside one `needs` list does not change run order.
- Several Steps are ready at the same time. The one written earliest in the Pipeline Definition runs next.
- A Step written earlier needs a Step written later, and a third Step has no Dependencies. The third Step is not given a Dependency on either of the others.
- No Step declares `needs`. Run order is definition order.
- A `needs` entry names an unknown id, names its own Step, repeats an id, or is empty.
- `needs` is a single string, a number, or any value other than a list. The definition is rejected.
- A list entry is not a Step id. The definition is rejected and names that Step.
- A cycle involves two Steps, and a cycle involves more than two Steps. Every Step in the cycle is named.
- One Step's failure occurs while another Step is already ready and does not need the failed Step. The ready Step does not start.
- One Step's failure occurs while a later Step still needs it. The waiting Step does not start.
- A selected Step command cannot be started. No further Step starts, and the failed Step has no Exit Code.
- A rejected definition already has a `.truss/run.log`. That file is unchanged and no command starts.
- A failed run still replaces `.truss/run.log` with the log of the run that started.
- Schema version is missing or is not `"1"`. The definition is rejected, as today.
- Unknown fields other than `needs` stay ignored. Only `needs` declares Dependencies.
- The same workflow is run on Linux, macOS, and Windows. Dependency rules do not change by host. Step `run` strings remain host-shell strings.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A Step MAY declare Dependencies with an optional `needs` field. The field lists the `id`s of the Steps that must succeed before that Step may start. The field name is `needs`.
- **FR-002**: A Step that omits `needs`, and a Step whose `needs` list is empty, MUST have no Dependencies. A Step's position in the Pipeline Definition MUST NOT give it a Dependency on the Step written before it. Declared `needs` entries are the only Dependencies.
- **FR-003**: Schema version `"1"` is retained. A Pipeline Definition that adds `needs` keeps version `"1"`. A definition that does not contain `needs` MUST remain valid and MUST run as specified in FR-012.
- **FR-004**: Before any Step starts, TRUSS MUST reject a Pipeline Definition when any of the following is true: a `needs` entry names a Step id that is not in the definition; a Step's `needs` list includes its own id; the declared Dependencies contain a cycle; one `needs` list repeats the same Step id; `needs` is present and is not a list of Step ids. The error MUST name the offending Step. An unknown id and a repeated id MUST also be named. A cycle MUST name every Step in the cycle. A self-dependency MUST state that the Step depends on itself.
- **FR-005**: A definition rejected by FR-004, or by the existing load and validation rules, MUST start no Step, MUST write no new run log, and MUST leave an existing `.truss/run.log` unchanged.
- **FR-006**: After successful validation, TRUSS MUST run Steps one at a time. A Step MUST start only after every Step it needs has succeeded. When more than one Step may start, TRUSS MUST start the Step that appears earliest in the Pipeline Definition, and MUST repeat that selection until a Step fails or no Step remains.
- **FR-007**: Steps that are ready at the same time do not need one another. Running the earliest one in the Pipeline Definition is sequential execution for this feature, and it MUST NOT add a Dependency. A later feature MAY run those ready Steps at the same time without changing the meaning of `needs`.
- **FR-008**: When a started Step fails, TRUSS MUST mark the Pipeline Run failed and MUST start no further Step. Steps that did not start MUST be omitted from the run log and MUST NOT be shown as succeeded. This applies both to Steps that need the failed Step and to Steps that were ready and do not need it.
- **FR-009**: The run log MUST keep the local run log format: a setup section, each started Step with its id, start time, output, presented-line times, finish time, duration, and Exit Code or start-failure diagnostic, and a closing summary. The log MUST show started Steps in the order they started. The log MUST NOT add the `needs` list, or any other listing of Dependencies, to a Step.
- **FR-010**: A Step command Exit Code of 0 MUST mark that Step succeeded. A non-zero Exit Code MUST mark that Step failed and MUST be preserved in the run log. A command that cannot be started MUST mark that Step failed, with a diagnostic and no Exit Code, and MUST stop the run as in FR-008.
- **FR-011**: `truss run` MUST keep its current load behavior. No path loads `truss.yaml` from the invoker's working directory. A path loads that file. The process exits 0 when the Pipeline Run succeeded and the latest run log was saved. The process exits non-zero when the definition was rejected, the environment or log location was not usable, or the Pipeline Run failed.
- **FR-012**: When no Step in a Pipeline Definition contains `needs`, TRUSS MUST run Steps sequentially in definition order, MUST fail the Pipeline Run on the first failed Step, and MUST produce the run log and process exit already required by the host pipeline engine and the local run log. Those existing acceptance outcomes MUST remain unchanged.
- **FR-013**: This feature MUST NOT execute any command other than the Step `run` strings of a Pipeline Run that has started.
- **FR-014**: Each Step Run MUST still execute in the invoker's working directory and MUST inherit the invoker's environment. This feature MUST NOT add per-Step environment, working directory, image, cache, matrix, or artifacts.

### Quality, Safety & Compatibility *(mandatory when applicable)*

- **Testing**: Each user story MUST remain independently testable, and each story MUST be implemented test-first: a failing test for that observable outcome exists before the behavior is implemented. Story 1 verifies dependency order and the ready-step tie-break. Story 2 verifies each invalid graph with zero command execution and an unchanged latest run log. Story 3 verifies that a failure starts no further Step. Story 4 verifies that a definition without `needs` matches the existing host pipeline and local run log behavior.
- **Process execution**: Only the Step `run` strings of a started Pipeline Run MAY be executed. Exit Codes MUST be preserved. A failed Step MUST fail the Pipeline Run, and no further Step MAY start. Cancellation, timeouts, retries, and conditional execution are out of scope. This feature does not define cancellation behavior.
- **Secrets**: `needs` carries Step ids, not secret material. The setup section and summary MUST NOT include environment variable names, environment variable values, or secret material. Output a Step command itself writes remains part of that Step's log.
- **Cross-platform**: Dependency validation and run order are the same on Linux, macOS, and Windows. The latest run log path remains `.truss/run.log` in the invoker's working directory. Step `run` strings remain host-shell strings. One Pipeline Definition is not required to be portable across host shells.
- **Compatibility**: This feature adds optional `needs` to schema version `"1"` and, when `needs` is used, replaces definition-order execution with the order in FR-006. The version stays `"1"` because `needs` is optional, every current definition remains valid without an edit, and no release has been tagged. A definition that uses `needs` is still version `"1"`. Definitions that do not contain `needs` keep sequential definition-order execution, fail-fast omission of later Steps, the run log contract, load behavior, and process exit codes. Unknown fields other than `needs` remain ignored. No released contract is broken, because no version has been tagged.
- **Portability**: This feature MUST NOT bind TRUSS to a CI provider, IDE, agent platform, operating system, or SDK language.

### Key Entities *(include if feature involves data)*

- **Step**: One unit of work in a Pipeline, identified by `id`, with a Command (`run`) and an optional `needs` list. The list names the Steps that must succeed before this Step may start.
- **Dependency**: The relationship declared by one `needs` entry. The named Step must succeed before the declaring Step may start. File order is not a Dependency.
- **Execution Graph**: The Dependencies declared by the Steps of one Pipeline Definition. It is valid only when every entry names another Step, no Step needs itself, no id is repeated in one list, and no cycle exists.
- **Execution Plan**: The sequential order in which a validated Pipeline will run for this feature. It honors the Execution Graph, and it chooses the earliest Step in the Pipeline Definition when several Steps are ready. That choice does not add a Dependency.
- **Pipeline Run**: One execution of a validated Pipeline. It succeeds when every Step has started and succeeded. It fails when a started Step fails, and the Steps that did not start remain outside the run.
- **Run Log**: The text of one Pipeline Run, unchanged in form from the local run log. Started Steps appear in the order they started. Dependencies are not printed.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On every successful Pipeline Run that declares Dependencies, 100% of started Steps appear in an order where each Step starts after every Step it needs has succeeded. Whenever more than one Step could have started, the one written earlier in the Pipeline Definition is the one that started next. The scenarios in User Story 1 produce the orders `build` then `test`; `lint` then `test` then `package`; `build` then `report` then `scan`; and `lint` then `package` then `publish`.
- **SC-002**: For 100% of the invalid-graph cases in FR-004, the developer sees an error that names the offending Step, 0 Step commands run, and an existing latest run log is unchanged.
- **SC-003**: When a Step fails, 0 further Steps start, the saved run log contains only Steps that started, and 0 omitted Steps appear as succeeded. The failed Step's Exit Code, or its start error when no Exit Code exists, is present, and the log names the failed Step.
- **SC-004**: For 100% of Pipeline Definitions that do not contain `needs`, run order, fail-fast behavior, run log text, load behavior, and process exit codes match the outcomes already required by the host pipeline engine and the local run log.
- **SC-005**: Across successful and failed runs that use Dependencies, the run log adds 0 lines whose purpose is to display a `needs` list. Every started Step still shows its start time, output, finish time, duration, and Exit Code or start error, and the summary still reports status, the number of Steps that started, the run's duration, and the time the run finished.

## Assumptions

- The actor is a developer running TRUSS on their own machine. The host pipeline engine and the local run log already load, validate, run, and record a schema version `"1"` pipeline.
- The public field name is `needs`. It is the list of Step ids that must succeed first. `depends_on` is not accepted as that field; an unknown field of that name stays ignored, as other unknown fields do.
- Schema version `"1"` is retained. A version bump would force every existing definition to change its version string even when it does not use Dependencies. That contradicts the requirement that those definitions keep working unchanged. No tagged release exists whose readers must be distinguished by a new version.
- The run log does not show Dependencies. The developer reads Dependencies in the Pipeline Definition. The log answers which Steps ran, in which order, and how each one ended. Adding the list would change the current log format.
- When a Step fails, the whole Pipeline Run stops, including Steps that do not need the failed Step. Letting those Steps continue is concurrent or conditional execution, and both are out of scope.
- `needs` compares Step ids exactly. A difference in case is an unknown id.
- An empty `needs` entry is not a Step id and is rejected.
- The order of ids inside a `needs` list is not meaningful. Repeating an id is rejected rather than collapsed.
- Duration and timestamps stay as the local run log defines them. The summary counts Steps that started. Omitted Steps are not counted as ran.
- Status values shown for a finished run remain `succeeded` and `failed`. Omitted Steps are not reported as `skipped` or `cancelled`.
- Glossary terms in `docs/GLOSSARY.md` are the vocabulary for this specification.
- Out of scope: concurrent execution, cancellation, timeouts, retries, conditional or run-on-failure Steps, per-Step environment or working directory, matrix builds, caching, artifacts, container or remote execution, a daemon, an API, an SDK, and a command for visualizing the Execution Graph.
