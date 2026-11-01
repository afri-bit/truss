# Feature Specification: Step Concurrency

**Feature Branch**: `claude/truss-cursor-to-claude-18fzh9` (assigned by the session; the constitution's name for it would be `feature/004-step-concurrency`)  
**Created**: 2026-11-01  
**Status**: Draft  
**Input**: User description: "Add concurrent step execution. The Pipeline Definition declares a concurrency limit; the default is 1, so a pipeline with no limit stays sequential. A higher limit lets TRUSS start Steps whose Dependencies have all succeeded, up to the limit at the same time, in definition order. A failure still fails the whole Pipeline Run: running Steps are stopped, Steps that have not started never run, and independent branches do not continue. The run log stays readable while Steps run together. Invalid limits are rejected before any command starts."

## Clarifications

### Session 2026-11-01

- Q: When a running Step is stopped because another Step failed, must TRUSS also end every process that the Step's command started, or only the Step's own command? → A: The whole process tree, on Linux, macOS, and Windows.
- Q: Should TRUSS now reject pipeline files that contain a field it does not recognise, instead of ignoring it? → A: Yes, reject unknown fields in this feature, as `docs/MILESTONE.md` says. This replaces spec 003's rule that unknown fields stay ignored.
- Q: Should TRUSS stop a running Step immediately and forcefully, or ask politely first and force it after a wait? → A: Immediately and forcefully, the same way on all three hosts. Commands get no chance to clean up.
- Q: How should the run log mark a Step that was stopped because another Step failed? → A: Keep the normal finish line and add one line, `step <id> stopped because step <other> failed`, just before it.
- Q: When should TRUSS end leftover background processes a Step started? → A: When that Step's shell exits. A Step can no longer leave a process running for a later Step; this also applies to sequential runs.
- Q: Should an interrupt (Ctrl-C or SIGTERM) on `truss` force-end all running Steps? → A: Yes. All running Steps are ended with their whole process trees, `truss` exits non-zero, no log is saved for the interrupted run, and no `cancelled` status appears.
- Q: When the host reports no Exit Code for a Step (killed by a signal), what does the log show? → A: A diagnostic and no Exit Code, for every Step, never a made-up `-1`.
- Q: A Step that reads from the terminal (password prompts) hangs on Linux and macOS once Steps run in their own process groups. How should TRUSS handle that? → A: Document it as a limitation. Pipelines are non-interactive; Ctrl-C still ends a hung Step.
- Q: How is the implementation split? → A: Two pull requests: first process-tree stopping and a thread-safe run log, then the schema field, scheduler, CLI, and docs.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run independent steps at the same time (Priority: P1)

A developer has a pipeline with Steps that do not need one another, such as `lint` and `test`. They raise the concurrency limit in the Pipeline Definition. TRUSS starts Steps whose Dependencies have all succeeded, up to the limit at once, and starts the next ready Step as soon as a running Step finishes. A Step still starts only after every Step it needs has succeeded. When several Steps are ready and only some can start, the ones written earliest in the Pipeline Definition start first.

**Why this priority**: Running ready Steps together is the purpose of this feature. Log readability, failure handling, and validation matter only once Steps can run together.

**Independent Test**: Run a definition with limit 2 and two Steps that need nothing, where each Step's command waits until it sees that the other Step has started before it exits. Confirm the run succeeds. Under sequential execution that pair would never finish, so success proves both ran at the same time.

**Acceptance Scenarios**:

1. **Given** limit 2 and two Steps `a` and `b` with no Dependencies, each waiting for the other to start, **When** the developer runs it, **Then** both start before either finishes and the Pipeline Run succeeds.
2. **Given** limit 2 and Steps `a`, `b`, `c` with no Dependencies, written in that order, **When** the developer runs it, **Then** `a` and `b` start first, and `c` starts only after `a` or `b` has finished.
3. **Given** limit 3 and Steps `build`, `lint`, and `test` (needs `build`), **When** the developer runs it, **Then** `build` and `lint` start first, `test` starts only after `build` has succeeded, and `test` does not wait for `lint`.
4. **Given** limit 2 and Steps `a`, `b` (needs `a`), `c`, `d` (needs `c`), **When** the developer runs it, **Then** `a` and `c` start first, and `b` and `d` each start after the Step they need has succeeded.
5. **Given** a limit larger than the number of Steps that can ever be ready together, **When** the developer runs it, **Then** TRUSS starts every ready Step and does not wait for more.
6. **Given** limit 2 and Steps whose Dependencies force a strict chain, **When** the developer runs it, **Then** they run one at a time in dependency order.

---

### User Story 2 - Read one log while steps run together (Priority: P2)

A developer reads the run log while several Steps run at once, and opens the saved latest run log afterward. The log is one text. Every line a Step writes is labelled with that Step's id, so output from different Steps can be told apart even when it alternates. Every line keeps its local timestamp, and timestamps never go backwards. The saved text is the same text shown during the run.

**Why this priority**: Concurrency that produces an unreadable log would make failures harder to diagnose than sequential runs. The log contract from the local run log must survive.

**Independent Test**: Run limit 2 with two Steps that each print several lines, alternating with short waits. Confirm each output line begins with its Step's id label, no line mixes text from two Steps, each Step's own lines are in the order that Step wrote them, timestamps do not go backwards, and the saved log equals the log shown.

**Acceptance Scenarios**:

1. **Given** two Steps running at the same time that each print several lines, **When** the developer reads the log, **Then** every output line carries its Step's id and one complete line of that Step's output, and lines from different Steps may alternate.
2. **Given** a Step that writes a partial line and then finishes the line later, **When** other Steps write meanwhile, **Then** the partial text appears as one complete line, not split by another Step's output.
3. **Given** a run with Steps that overlap in time, **When** the developer reads the log, **Then** each started Step shows its id, start time, output, finish time, duration, and Exit Code or start-failure diagnostic, and the log lists Steps in the order they started.
4. **Given** any Pipeline Run that finished, succeeded or failed, **When** the developer opens `.truss/run.log`, **Then** the saved text is the log shown for that run. (An interrupted run saves no log; see FR-017.)
5. **Given** a finished run, **When** the developer reads the summary, **Then** it reports the status, the number of Steps that started, the run's duration, and the time the run finished, and the setup section still shows the environment, operating system, architecture, host shell, and working directory.
6. **Given** the log, **When** the developer reads it, **Then** it does not list environment variable names or values and does not display the concurrency limit or any Dependencies.

---

### User Story 3 - Stop the whole run when a step fails (Priority: P3)

A developer runs a pipeline with several Steps running together and one of them fails. TRUSS fails the Pipeline Run, stops the Steps that are still running, never starts a Step that has not started, and saves a log of the Steps that started. Independent branches do not continue.

**Why this priority**: Fail-fast is the existing contract. Concurrency must not weaken it by letting other branches finish or by leaving commands running after TRUSS exits.

**Independent Test**: Run limit 2 with a Step that exits non-zero shortly after start, a long-running Step that would print a distinctive marker if it ever finished, and a third Step that needs nothing and would print its own marker. Confirm the run fails, the process exits non-zero, neither marker appears, the long-running Step is shown stopped with a finish time and duration, the third Step is absent, and no Step command is still running when `truss` exits.

**Acceptance Scenarios**:

1. **Given** limit 2 with `a` failing and `b` still running, **When** `a` fails, **Then** `b` is stopped, the Pipeline Run is failed, no further Step starts, and `truss` exits non-zero.
2. **Given** a Step that is stopped because another Step failed, **When** the developer reads the log, **Then** that Step shows the output it produced before it was stopped, a line `step <id> stopped because step <other> failed`, and then its finish line with a finish time, a duration, and the Exit Code the host reports for it, or a diagnostic and no Exit Code when the host reports none.
3. **Given** a failure while other Steps are ready but have not started, **When** the developer reads the log, **Then** those Steps are absent and are not shown as succeeded.
4. **Given** a Step whose command cannot be started while another Step is running, **When** the Pipeline Run ends, **Then** the unstartable Step shows a diagnostic and no Exit Code, the running Step is stopped, and no further Step starts.
5. **Given** two Steps that fail at about the same time, **When** the developer reads the log, **Then** both appear with their own Exit Codes, and the run's error names the Step whose failure TRUSS observed first.
6. **Given** a Step that is stopped, **When** the run has ended, **Then** its command and every process that command started, directly or indirectly, have ended, on Linux, macOS, and Windows, and none keeps writing to the log.
7. **Given** a failed Pipeline Run that used concurrency, **When** the developer opens the latest run log, **Then** the saved text is the log just shown, the summary reports failure, and the error names the pipeline and the failed Step.
8. **Given** Steps running and the developer interrupts `truss` (Ctrl-C on every host; SIGTERM on Linux and macOS), **When** `truss` exits, **Then** every running Step and its whole process tree has ended, no further Step started, `truss` exits non-zero, the existing latest run log is unchanged, and no `cancelled` status is reported.
9. **Given** a Step killed by a signal that TRUSS did not send, **When** the developer reads the log, **Then** the finish line shows a diagnostic and no Exit Code, not `-1`.

---

### User Story 4 - Reject an invalid concurrency limit (Priority: P4)

A developer writes a concurrency limit TRUSS cannot honor: zero, a negative number, a number with a fraction, text, a list, or another value that is not a whole number of at least 1. TRUSS explains the problem, starts no command, and leaves any existing latest run log unchanged.

**Why this priority**: A silently wrong limit would run commands at a rate the developer did not ask for.

**Independent Test**: For each invalid value, run the definition while a latest run log already exists. Confirm an error names the field and the offending value, no Step command runs, and the stored log is byte-for-byte unchanged.

**Acceptance Scenarios**:

1. **Given** a limit of `0`, `-1`, `1.5`, `"two"`, `true`, or a list, **When** the developer runs it, **Then** TRUSS reports a validation error that names the concurrency field, starts no Step, writes no new run log, and leaves an existing latest run log unchanged.
2. **Given** an empty or null limit, **When** the developer runs it, **Then** TRUSS rejects it the same way, rather than treating it as the default.
3. **Given** a valid limit combined with an invalid Dependency graph, **When** the developer runs it, **Then** the existing Dependency errors are reported, and no Step starts.
4. **Given** a misspelled field name for the limit, such as `concurency`, **When** the developer runs it, **Then** TRUSS rejects the definition because of the unknown field, names the field and where it appears, starts no Step, and leaves an existing latest run log unchanged.
5. **Given** an unknown field at the top level, under `pipeline`, or in a Step, such as `need` for `needs`, **When** the developer runs it, **Then** TRUSS rejects it the same way.
6. **Given** a definition that uses only known fields (`version`, `pipeline.name`, `pipeline.concurrency`, and per Step `id`, `run`, `needs`), **When** the developer runs it, **Then** it is accepted.

---

### User Story 5 - Leave pipelines without a limit unchanged (Priority: P5)

A developer runs a Pipeline Definition that does not declare a concurrency limit, or declares a limit of 1. Steps run one at a time, in the order spec 003 defines, the first failed Step fails the run, and the run log, load behavior, and process exit codes stay as they are today.

**Why this priority**: Existing pipelines are the regression baseline.

**Independent Test**: Run the same successful and failed definitions that spec 003 uses, with no limit and again with limit 1. Confirm run order, fail-fast omission, saved log text shape, and exit codes match spec 003.

**Acceptance Scenarios**:

1. **Given** a valid definition with no limit whose commands succeed, **When** the developer runs it, **Then** Steps run one at a time, the run log matches the log contract of specs 002 and 003, and the process exits 0.
2. **Given** a limit of exactly 1, **When** the developer runs it, **Then** the behavior is identical to omitting the limit.
3. **Given** a definition with no limit whose second Step fails, **When** the developer runs it, **Then** later Steps are omitted, the log is saved, and the process exits non-zero.
4. **Given** `truss run` with no path or with a path, **When** the developer runs it, **Then** load behavior is unchanged.
5. **Given** a Step whose command starts a background process and then exits successfully, **When** the Step's shell exits, **Then** TRUSS ends that background process, so no later Step can rely on it. This is the one deliberate change to sequential behavior (see Compatibility).

---

### Edge Cases

- An unknown field appears anywhere in the file (top level, under `pipeline`, or in a Step): the definition is rejected before any Step starts.
- The limit is omitted: it is 1.
- The limit is larger than the number of Steps: every ready Step may run together.
- Only one Step is ready at a time because of Dependencies: it runs alone even with a high limit.
- Several Steps are ready and slots are scarce: the earliest written start first, and a freed slot goes to the earliest ready Step.
- A running Step finishes successfully and unlocks more than one Step: they start in definition order, up to the limit.
- A Step writes to standard output and standard error at the same time: both appear under its id, as one text.
- A Step writes output with no final newline: the partial line appears once, complete, when the Step finishes.
- Two Steps start in the same second: their timestamps may match, and log order is the order they started.
- A Step fails immediately while another has not yet started: the second never starts.
- A Step fails after another succeeded and unlocked a dependent: the dependent does not start if the failure came first.
- A Step cannot be started while others run: handled as a failure (User Story 3, scenario 4).
- Every Step is independent and the limit is 1: sequential, definition order.
- The latest run log already exists and the new run is rejected before start: it is unchanged.
- The same pipeline runs on Linux, macOS, and Windows: validation, ordering, and log shape do not change by host; how a running Step is stopped is specified under Cross-platform.
- A Step that waits for input from the terminal (a password prompt) is stopped by the terminal on Linux and macOS and waits until `truss` is interrupted. Pipelines are non-interactive.
- A stopped Step gets no chance to clean up after itself (temporary files, locks). Commands that need cleanup must not rely on running to the end.
- The developer interrupts `truss` with Ctrl-C or SIGTERM: all running Steps are ended, `truss` exits non-zero, and the existing latest run log is unchanged. No `cancelled` status is reported.
- A Step starts a background process and exits: the background process is ended when the Step's shell exits.
- A process started by a Step detaches itself from the Step's process group or job (for example `setsid` on Unix): TRUSS cannot guarantee it ends. This is a documented limitation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A Pipeline Definition MAY declare a concurrency limit with an optional `concurrency` field under `pipeline`. The value is the largest number of Steps TRUSS may run at the same time.
- **FR-002**: When the field is omitted, the limit MUST be 1. A limit of 1 MUST behave exactly as an omitted limit.
- **FR-003**: Before any Step starts, TRUSS MUST reject a `concurrency` value that is not a whole number of at least 1, including zero, negative numbers, numbers with a fraction, text, booleans, lists, mappings, and an empty or null value. The error MUST name the `concurrency` field and the offending value. A rejected definition MUST start no Step, MUST write no new run log, and MUST leave an existing `.truss/run.log` unchanged.
- **FR-004**: After successful validation, TRUSS MUST start a Step only when every Step it needs has succeeded and fewer Steps than the limit are running. When several Steps may start, TRUSS MUST start the ones written earliest in the Pipeline Definition first, and MUST repeat this selection each time a running Step finishes.
- **FR-005**: A Step with no Dependencies MUST NOT wait for the Step written before it. Declared `needs` entries remain the only Dependencies, as in spec 003.
- **FR-006**: When a started Step fails, including a command that cannot be started, TRUSS MUST mark the Pipeline Run failed, MUST start no further Step, and MUST stop the Steps that are still running. Stopping MUST be immediate and forceful, with no polite shutdown request and no grace period, and MUST work the same way on Linux, macOS, and Windows. Steps that did not start MUST be omitted from the run log and MUST NOT be shown as succeeded. This applies equally to Steps that need the failed Step and to ready Steps that do not.
- **FR-007**: A Step stopped by FR-006 MUST appear in the run log with its output so far, one line `step <id> stopped because step <other> failed` naming the Step that failed on its own, and then the normal finish line with a finish time, a duration, and the Exit Code the host reports for it. When the host reports none, the finish line shows a diagnostic and no Exit Code. A stopped Step MUST NOT be reported as succeeded, and TRUSS MUST NOT add a new status such as `cancelled`. The `stopped` line is the only new line type this feature adds to the log.
- **FR-008**: The run's error MUST name the pipeline and the Step whose failure TRUSS observed first. A failure of any Step MUST NOT be swallowed: every started Step's own Exit Code or start diagnostic MUST appear in the log.
- **FR-009**: The run log MUST remain one text that is also what is saved to `.truss/run.log`. Every line a Step writes MUST be one complete line labelled with that Step's id and MUST keep its own local timestamp. Lines from different Steps MAY alternate. A Step's own lines MUST appear in the order that Step wrote them. Timestamps MUST NOT move earlier than the previous line.
- **FR-010**: The log MUST keep the setup section, each started Step's id, start time, output, finish time, duration, and Exit Code or start diagnostic, and the closing summary of status, number of Steps that started, duration, and finish time. Started Steps MUST be listed in the order they started. The log MUST NOT show the concurrency limit, any Dependencies, or environment variable names or values.
- **FR-011**: A Step Exit Code of 0 MUST mark that Step succeeded; any other Exit Code MUST mark it failed and MUST be preserved in the log. When the host reports no Exit Code for a Step that started, such as a Step ended by a signal, the finish line MUST show a diagnostic and no Exit Code, for every Step. TRUSS MUST NOT print a made-up Exit Code such as `-1`.
- **FR-012**: When no `concurrency` field is declared, TRUSS MUST run Steps and produce the run log and the process exit exactly as specs 001, 002, and 003 require, except for FR-011 (no made-up Exit Code), FR-014 (leftover processes), and FR-016 (unknown fields).
- **FR-013**: `truss run` MUST keep its current load behavior and process exit codes. The process exits 0 only when the Pipeline Run succeeded and the latest run log was saved.
- **FR-014**: When `truss` exits, on a successful or a failed run, no Step command that TRUSS started, and no process such a command started, MAY still be running. A Step stopped by FR-006 MUST have its whole process tree ended on Linux, macOS, and Windows. When a Step's shell exits, on any run and with any limit, TRUSS MUST end every process that Step started and that is still running.
- **FR-015**: This feature MUST NOT execute any command other than the Step `run` strings of a Pipeline Run that has started. Each Step MUST still run in the invoker's working directory with the invoker's environment. This feature MUST NOT add per-Step environment, working directory, timeouts, retries, or caching.
- **FR-016**: Before any Step starts, TRUSS MUST reject a Pipeline Definition that contains a field it does not recognise at the top level, under `pipeline`, or in a Step. The recognised fields are `version`, `pipeline.name`, `pipeline.concurrency`, and per Step `id`, `run`, and `needs`. The error MUST name the unknown field and the Step or section it appears in. Rejection MUST start no Step, write no new run log, and leave an existing `.truss/run.log` unchanged. This replaces the rule in spec 003 that unknown fields stay ignored.
- **FR-017**: When the developer interrupts `truss` (Ctrl-C on every host; SIGTERM on Linux and macOS, where Windows has no equivalent) while a Pipeline Run is in progress, TRUSS MUST start no further Step, MUST end every running Step with its whole process tree as in FR-006, and MUST exit non-zero. It MUST NOT save a run log for the interrupted run, MUST leave an existing `.truss/run.log` unchanged, and MUST NOT report a `cancelled` status.

### Quality, Safety & Compatibility *(mandatory when applicable)*

- **Testing**: Each user story MUST remain independently testable and MUST be implemented test-first: a failing test for the observable outcome exists before the behavior is implemented. Story 1 uses commands that wait for each other, so a sequential implementation cannot pass. Story 2 checks line labels, completeness of lines, per-Step order, timestamp monotonicity, and equality of the shown and saved log. Story 3 checks that no marker from a stopped or unstarted Step appears and that neither a Step command nor a process it started outlives `truss`. Story 4 checks every invalid value with zero command execution and an unchanged log. Story 5 compares with the spec 003 outcomes. Concurrency tests MUST be repeatable and MUST NOT rely on sleeps as synchronization.
- **Process execution**: Only Step `run` strings of a started Pipeline Run MAY be executed. An interrupt of `truss` ends all running Steps (FR-017). Exit Codes MUST be preserved. A failed Step fails the Pipeline Run, running Steps are stopped, and no further Step starts. Stopping a Step MUST NOT hide its failure. Cancellation as a reported status, timeouts, retries, and conditional execution are out of scope.
- **Secrets**: The setup section, summary, and any TRUSS-written line MUST NOT include environment variable names, values, or secret material. Output a Step command itself writes remains part of that Step's lines.
- **Cross-platform**: Validation, ordering, and log shape MUST be the same on Linux, macOS, and Windows. An interrupt is Ctrl-C on every host and SIGTERM on Linux and macOS only; the interrupt handling path (cancelling the run) MUST be tested on all three hosts, and the real console Ctrl-C is verified by hand on Windows. How a running Step is stopped depends on the host; the plan MUST name the mechanism for each host and the tests MUST cover each host in CI. A `run` string remains a host-shell string. One Pipeline Definition is not required to be portable across shells.
- **Compatibility**: This feature adds the optional `pipeline.concurrency` field to schema version `"1"`. Every current definition that uses only recognised fields remains valid and runs as before. A definition that contains an unrecognised field was accepted before and is now rejected (FR-016); this is a deliberate, milestone-directed tightening, and no release has been tagged. Two further sequential-mode changes are deliberate: a Step can no longer leave a background process running after its shell exits (FR-014), and a signal death logs a diagnostic instead of `-1` (FR-011). The version stays `"1"` because the field is optional and no release has been tagged. When the field is used, the run log gains concurrent output in the shape described in FR-009 and the `stopped` line from FR-007; nothing else is added.
- **Portability**: This feature MUST NOT bind TRUSS to a CI provider, IDE, agent platform, OS, or SDK language.

### Key Entities *(include if feature involves data)*

- **Concurrency limit**: The largest number of Steps that may run at the same time in one Pipeline Run. Optional. Default 1. A whole number of at least 1.
- **Step**: One unit of work with an `id`, a Command (`run`), and optional `needs`.
- **Ready Step**: A Step that has not started, whose every needed Step has succeeded.
- **Running Step**: A Step that has started and not yet finished.
- **Execution Plan**: The rule by which ready Steps start: Dependencies first, then definition order, never more than the limit at once.
- **Pipeline Run**: One execution of a validated Pipeline. It succeeds when every Step has started and succeeded. It fails when a started Step fails; Steps that did not start stay outside the run.
- **Run Log**: One text of a Pipeline Run, shown live and saved unchanged. Output lines are labelled by Step id.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With limit 2, a pair of independent Steps that each wait for the other to start completes successfully in 100% of runs; a sequential run of the same definition does not complete.
- **SC-002**: In 100% of successful runs, a Step starts only after all Steps it needs have succeeded, no more Steps run at once than the limit, and when several are ready the earliest written start first.
- **SC-003**: In 100% of runs with overlapping Steps, every output line in the log is one complete line labelled with exactly one Step id, each Step's lines are in the order it wrote them, and no timestamp is earlier than the one before it.
- **SC-004**: The saved `.truss/run.log` equals the log shown, for successful and failed runs, 100% of the time.
- **SC-005**: When a Step fails, 0 Steps that had not started begin, no marker that a stopped or unstarted Step would have produced after the failure exists, every stopped Step's log ends with its `stopped` line and finish line, the process exits non-zero, and 0 Step commands or processes they started are still running once `truss` has exited.
- **SC-006**: For 100% of invalid limit values listed in FR-003 and 100% of unknown fields (FR-016), the developer sees an error naming the `concurrency` field, 0 Step commands run, and an existing latest run log is unchanged.
- **SC-007**: For 100% of definitions with no limit, and for limit 1, run order, fail-fast omission, run log text, load behavior, and exit codes match specs 001 to 003.
- **SC-008**: Linux, macOS, and Windows all pass the same behavior tests for Stories 1 to 5 in CI.

## Assumptions

- The actor is a developer running TRUSS on their own machine. Specs 001, 002, and 003 already load, validate, run, and record a schema version `"1"` pipeline with `needs`.
- The field is `pipeline.concurrency`. It is a property of the Pipeline, next to `name`, not of a Step, and there is no command-line override in this release.
- The limit is a plain whole number; there is no `auto` or CPU-based value and no upper bound. A very large limit behaves like "as many as are ready".
- Output labelling uses the existing `step <id> | ` line form from the local run log, so a sequential log and a concurrent log read the same way.
- The live log and the saved log are the same text; concurrency does not introduce a second format.
- Status values stay `succeeded` and `failed`. A stopped Step is `failed` with the Exit Code or diagnostic described in FR-007; it is not reported as `cancelled` or `skipped`.
- Pipelines are non-interactive: a Step cannot rely on reading from the terminal.
- Stopping uses no grace period, so TRUSS does not wait for a Step to finish on its own after a failure.
- "Observed first" is by the order TRUSS records the failures, not by wall-clock time of the commands.
- An interrupt ends the run without a reported status. Cancellation as a reported status stays out of this release (`docs/MILESTONE.md`).
- Glossary terms in `docs/GLOSSARY.md` are the vocabulary for this specification.
- Out of scope: cancellation as a reported status (an interrupt ends the run without one), per-Step timeouts or retries, per-Step environment or working directory, matrix builds, caching, artifacts, container or remote execution, a daemon, an API, an SDK, and a command for visualizing the Execution Graph.
