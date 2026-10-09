# Feature Specification: Host-Executed Pipeline Engine

**Feature Branch**: `001-host-pipeline-engine`  
**Created**: 2026-09-16  
**Status**: Draft  
**Input**: User description: "Host-executed pipeline engine: load a YAML pipeline definition, parse it into a pipeline model, validate it, plan sequential execution, run each step's command on the host executor, capture output and exit codes, stop on step failure, return a structured result, and invoke the flow through a thin CLI."

## Clarifications

### Session 2026-10-09

- Q: How is a Step `run` value executed? → A: One string, executed by the host shell
- Q: What working directory and environment does a Step inherit? → A: The invoker's working directory and environment
- Q: Which file does `truss run` load when no path is given? → A: `truss.yaml` in the invoker's working directory. An explicit path may use any filename.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a valid local pipeline (Priority: P1)

A developer writes a Pipeline Definition in YAML, runs it locally with the CLI, and sees a structured Result: which Steps ran, in what order, whether each succeeded, and the output and Exit Code of each Step Run.

**Why this priority**: This is the first product milestone. Without a complete local run, TRUSS cannot replace a remote CI loop for everyday pipeline work.

**Independent Test**: Provide a valid Pipeline Definition with two successful host commands. Run it through the CLI. Confirm both Step Runs appear in definition order, each with status succeeded, Exit Code 0, and captured output, and that the Pipeline Run succeeded.

**Acceptance Scenarios**:

1. **Given** a valid Pipeline Definition with one Step whose command succeeds, **When** the developer runs it with `truss run <path>`, **Then** TRUSS parses and validates the definition, executes that Step on the host, and reports a succeeded Pipeline Run with that Step's status, Exit Code, and output.
2. **Given** a valid Pipeline Definition whose filename is not `truss.yaml`, **When** the developer runs `truss run` with that path, **Then** TRUSS loads that file.
3. **Given** a valid Pipeline Definition with multiple Steps whose commands succeed, **When** the developer runs it, **Then** Steps execute one after another in definition order and the Result lists every Step Run in that same order.
4. **Given** a valid Pipeline Definition, **When** the developer inspects the Result after the run, **Then** the Result includes Pipeline Run status plus each executed Step Run's identifier, status, Exit Code, and output.
5. **Given** a valid Pipeline Definition at `truss.yaml` in the invoker's working directory, **When** the developer runs `truss run` with no path, **Then** TRUSS loads that file and runs it.

---

### User Story 2 - Reject invalid definitions without executing (Priority: P2)

A developer points the CLI at a Pipeline Definition that is not valid. TRUSS reports a clear validation or parse error and does not start a Pipeline Run or execute any Step command.

**Why this priority**: Executing an invalid definition is unsafe and confusing. Validation must be independently useful even before host execution exists.

**Independent Test**: Submit each specified invalid case (unreadable or invalid YAML, missing required fields, empty Steps, duplicate Step IDs, empty or missing `run`, missing or unsupported schema version). Confirm a user-visible error and that no Step command runs.

**Acceptance Scenarios**:

1. **Given** a file that is not valid YAML, **When** the developer runs it, **Then** TRUSS reports a parse error and does not execute any command.
2. **Given** YAML that is missing a required field, has no Steps, has duplicate Step IDs, has an empty or missing `run`, or has a missing or unsupported schema version, **When** the developer runs it, **Then** TRUSS reports a validation error naming the problem and does not execute any command.
3. **Given** a path that does not resolve to a readable Pipeline Definition file, **When** the developer runs it, **Then** TRUSS reports a user-visible error and does not execute any command.
4. **Given** the invoker's working directory has no readable `truss.yaml`, **When** the developer runs `truss run` with no path, **Then** TRUSS reports a user-visible error and does not execute any command.

---

### User Story 3 - Stop on failure and inspect the failed step (Priority: P3)

A developer runs a Pipeline whose later Step would also do work. An earlier Step fails. TRUSS stops, marks the Pipeline Run failed, preserves the failed Step's Exit Code and output, and does not run later Steps.

**Why this priority**: Fail-fast sequential semantics are a public Execution Semantics contract. Developers must be able to diagnose the first failure without later Steps mutating the machine.

**Independent Test**: Run a three-Step definition where the second command exits non-zero. Confirm the first Step succeeded, the second failed with its real Exit Code and output, the third never ran and is not marked succeeded, and the Pipeline Run failed.

**Acceptance Scenarios**:

1. **Given** a valid Pipeline Definition where a Step command exits non-zero, **When** the developer runs it, **Then** that Step Run is failed, its Exit Code is the command's Exit Code, its output is captured, and the Pipeline Run is failed.
2. **Given** a valid Pipeline Definition with Steps after the failing Step, **When** the developer runs it, **Then** those later Steps do not execute and do not appear as succeeded.
3. **Given** a Step command that cannot be started, **When** the developer runs it, **Then** that Step Run is failed, the Result includes enough failure context to diagnose the start failure, later Steps do not execute, and the Pipeline Run is failed.
4. **Given** a Pipeline Run that failed, **When** the developer inspects the Result, **Then** they can identify which Step failed and why without needing a remote CI system.

---

### Edge Cases

- The given path does not exist, is not a file, or cannot be read.
- The file is empty or contains valid YAML that is not a Pipeline Definition.
- Schema version is present but not `"1"`.
- Pipeline name is missing or empty.
- Steps is missing, empty, or not a list.
- Two Steps share the same `id`.
- A Step `id` is missing or empty.
- A Step `run` is missing or empty.
- A Step command exits 0 with empty output.
- A Step command writes output and then exits non-zero.
- A Step command cannot be started on the host (for example, the program is not found).
- A Step `run` string contains shell syntax (quotes, pipes, or environment expansion). The host shell interprets that syntax.
- `truss run` is invoked with no path and `truss.yaml` is missing or unreadable in the invoker's working directory.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to run a Pipeline by invoking `truss run <path>` with a local Pipeline Definition file. The path MUST be used as given, and the filename MAY be any name.
- **FR-002**: When the user invokes `truss run` without a path, TRUSS MUST look for `truss.yaml` in the invoker's working directory and load it. If that file is missing or unreadable, TRUSS MUST report a user-visible error and MUST NOT execute any command. This default lookup applies only when no path is given.
- **FR-003**: TRUSS MUST load the Pipeline Definition from the given local YAML file and parse it into a Pipeline Model. Parsing MUST NOT execute any Step command.
- **FR-004**: If the file cannot be read or is not valid YAML, TRUSS MUST report a user-visible error and MUST NOT start a Pipeline Run.
- **FR-005**: Before execution, TRUSS MUST validate the Pipeline Model and reject at least: missing required fields, empty or missing Steps, missing or empty Pipeline name, missing or empty Step `id`, duplicate Step IDs, missing or empty `run`, and missing or unsupported schema version.
- **FR-006**: A valid v1 Pipeline Definition MUST include schema version `"1"`, a Pipeline name, and one or more Steps. Each Step MUST have a unique `id` and a `run` command. This milestone MUST NOT add optional definition fields (`env`, working directory, `depends_on`, image, cache, matrix, artifacts). Working directory and environment come from the invoker (FR-010).
- **FR-007**: After successful validation, TRUSS MUST produce an Execution Plan that runs Steps sequentially in definition order. This milestone MUST NOT use concurrent execution or a dependency graph.
- **FR-008**: The Host Executor MUST execute only the command explicitly specified on each Step. Implicit command execution is forbidden.
- **FR-009**: Each Step `run` value MUST be a single string executed by the host shell. It MUST NOT be an argument list. This is a Public Contract. The host shell interprets the string, including quotes, pipes, and environment expansion.
- **FR-010**: Each Step Run MUST execute in the invoker's working directory and MUST inherit the invoker's environment. The Pipeline Definition MUST NOT set either one in this milestone.
- **FR-011**: For every executed Step Run, TRUSS MUST capture status, Exit Code, and output, and MUST preserve the command's Exit Code without swallowing it.
- **FR-012**: A Step command Exit Code of 0 MUST mark that Step Run succeeded. A non-zero Exit Code MUST mark that Step Run failed. A command that cannot be started MUST mark that Step Run failed and MUST include diagnostic failure context in the Result.
- **FR-013**: When a Step Run fails, TRUSS MUST mark the Pipeline Run failed, MUST NOT execute later Steps, and MUST NOT mark those later Steps succeeded.
- **FR-014**: TRUSS MUST return a structured Result for the Pipeline Run and each executed Step Run, including status, Exit Code (when a command ran), output (when a command ran), and enough failure context to diagnose parse, validation, start, or command failures.
- **FR-015**: The CLI MUST be a thin interface: `truss run` loads, validates, executes, and reports the engine Result. Engine behavior MUST NOT depend on the CLI. The Pipeline Model MUST remain independent of which executor runs Steps. The Host Executor is the only executor in this milestone.
- **FR-016**: Parse and validation behavior MUST be independently testable without executing host commands. Host execution of a known command MUST be independently testable. The CLI success and failure contract MUST be independently testable.

### Quality, Safety & Compatibility *(mandatory when applicable)*

- **Testing**: Each user story MUST remain independently testable, and each story MUST be implemented test-first: a test that fails for the missing behavior exists before that behavior is implemented. Story 1 verifies a successful sequential run and Result contents. Story 2 verifies parse and validation errors with zero command execution. Story 3 verifies fail-fast behavior, Exit Code preservation, and that later Steps do not run.
- **Process execution**: Only commands specified on Steps MAY be executed. Exit Codes MUST be preserved. Failures MUST propagate to the Pipeline Run. Cancellation, timeouts, retries, and skip conditions are out of scope; this milestone does not define cancellation behavior.
- **Secrets**: This milestone has no secret store. TRUSS MUST NOT invent secret-echoing, credential logging, or extra diagnostic dumps of the host environment. Step output is captured as produced by the user-specified command. Results and diagnostics MUST NOT add environment dumps or credential material that the Step command did not write.
- **Cross-platform**: Pipeline Definitions use local filesystem paths. Exit Code 0 means success and non-zero means failure on every supported host. Each `run` value is a host-shell string, so quoting, path separators, and shell syntax follow that host's shell. The same Pipeline Definition is not required to be portable across different host shells in this milestone.
- **Compatibility**: YAML schema version `"1"`, `truss run`, sequential definition-order execution, and fail-fast host semantics are Public Contracts introduced by this feature. Incompatible changes after this contract is established MUST identify compatibility impact first.
- **Portability**: This feature MUST NOT bind TRUSS to a specific CI provider, IDE, agent platform, operating system, or SDK language. The same Pipeline Definition and Execution Semantics MUST apply for local host execution regardless of where the developer invokes the CLI.

### Key Entities *(include if feature involves data)*

- **Pipeline**: The named workflow the user wants executed. Contains one or more Steps.
- **Pipeline Definition**: The YAML representation of a Pipeline (schema version, name, Steps).
- **Pipeline Model**: The internal representation produced by the Parser from a Pipeline Definition. Used for Validation and planning. Not an execution record.
- **Pipeline Run**: One execution instance of a validated Pipeline. Has a Result and status.
- **Step**: One unit of work in a Pipeline, identified by `id`, with a Command (`run`).
- **Step Run**: One execution instance of a Step within a Pipeline Run. Executes in the invoker's working directory and environment. Has status, Exit Code, and output when executed.
- **Command**: The instruction on a Step that the Host Executor runs.
- **Execution Plan**: The sequential, definition-order plan for a validated Pipeline. No dependency graph in this milestone.
- **Host Executor**: The component that runs a Step Command on the machine running TRUSS.
- **Result**: The structured outcome of a Pipeline Run and its executed Step Runs (status, Exit Code, output, failure context).
- **Status**: For this milestone, a completed Pipeline Run or Step Run is `succeeded` or `failed`. Later unrun Steps are not marked succeeded and are not given `skipped` or `cancelled` (those statuses are out of scope).
- **Exit Code**: The numeric result of an executed host command. `0` is success; any other value is failure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer with a valid local Pipeline Definition can start a run with a single CLI invocation and, when the run finishes, see every executed Step's order, identity, status, Exit Code, and output.
- **SC-002**: 100% of the invalid-definition cases listed in FR-005, plus unreadable or invalid YAML, are reported to the user with no Step command executed.
- **SC-003**: When a Step fails, 0 later Steps execute, the failed Step's Exit Code and output are present in the Result, and the Pipeline Run is failed.
- **SC-004**: A developer can complete the loop "edit definition or command → run locally → observe Result → fix → run again" using only the local CLI, with no remote CI system, daemon, or worker.
- **SC-005**: For every successful or partially executed Pipeline Run, the reported Step Run order matches Pipeline Definition order for 100% of executed Steps.

## Assumptions

- The actor is a developer running TRUSS on their own machine. No authentication, multi-user access, or daemon is required.
- Schema version `"1"` is the only supported version in this milestone.
- Each Step `run` is one string executed by the host shell, matching `run: echo "Hello from TRUSS"`.
- Each Step Run uses the working directory and environment of the process that invoked the CLI.
- `truss run <path>` uses that path as given, including any filename. `truss run` with no path looks for `truss.yaml` in the invoker's working directory.
- Sequential execution means definition order, not an inferred dependency graph.
- Status values in this milestone are `succeeded` and `failed` for completed runs. `pending`, `running`, `cancelled`, and `skipped` are not part of the reported Result contract yet.
- Later Steps after a failure do not appear as succeeded Step Runs. They are omitted from executed Step Runs rather than reported as skipped.
- Validation occurs only as part of `truss run`. There is no validate-only CLI command in this milestone. Validation remains independently testable through the engine.
- The CLI reports the structured Result in human-readable form (Step order, identity, status, Exit Code, output, and errors). The `truss` process Exit Code is 0 when the Pipeline Run succeeded and non-zero when the definition cannot be loaded or validated or the Pipeline Run failed.
- A Step command that cannot be started is a failed Step Run, not a parse or validation error, because the definition was already accepted.
- Empty output on a successful command is valid and MUST be reported as empty, not as an error.
- Host execution uses the machine where the user invoked the CLI. Container and remote executors are out of scope.
- Out of scope: daemon, API, SDK, workers, remote or container executors, concurrent execution, dependency graphs, `depends_on`, cancellation, timeouts, retries, skip conditions, matrix builds, caching, artifacts, dashboards, CI provider bindings, secret stores, and persistence of historical Pipeline Runs.
- Optional Pipeline Definition fields listed in FR-006 stay out of this milestone.
- Glossary terms in `docs/GLOSSARY.md` are the vocabulary for this specification.
