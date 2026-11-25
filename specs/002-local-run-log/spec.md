# Feature Specification: Local Run Log

**Feature Branch**: `002-local-run-log`  
**Created**: 2026-10-26  
**Status**: Implemented  
**Input**: User description: "A developer running a pipeline locally needs a run log they can read while it executes and find again afterward. The log shows whether the environment was captured, the operating system and architecture, the host shell, and the working directory, then each step and a closing summary. Every occurrence and every step carries the time it happened. It does not add a secret store, declared variables, or a dump of the invoker's environment. The latest log is saved under .truss/ in the working directory. Help and version are available without starting a run."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read a run as it happens (Priority: P1)

A developer runs a valid pipeline and watches one run log from start to finish. Before any step output, the log states that the environment was captured and is ready, and it shows the operating system, architecture, host shell, and working directory, with the time that happened. Each step that runs is named with the time it started. Its output appears as the command produces it, and each of those lines shows the time TRUSS presented it. The step's finish time, duration, and Exit Code follow that output. The log closes with the Pipeline Run status, how many steps ran, the run's duration, and the time the run finished.

**Why this priority**: The run log is the product of this feature. Without a readable live log, saving a copy and documenting the CLI do not help the developer see what happened.

**Independent Test**: Run a valid two-step pipeline whose commands succeed. Confirm the setup section appears before any step output, both steps appear in definition order with a start time, their output, a finish time, duration, and Exit Code, every TRUSS line carries a timestamp, and the summary reports success, 2 steps, the run's duration, and the time the run finished.

**Acceptance Scenarios**:

1. **Given** a valid Pipeline Definition whose commands succeed, **When** the developer runs it, **Then** the run log begins with a setup section that states the environment was captured and is ready, shows the operating system, architecture, host shell, and working directory, and shows the time that setup occurred.
2. **Given** a valid Pipeline Definition with more than one successful Step, **When** the developer runs it, **Then** each Step appears in definition order with the time it started, that Step's output appears as it is produced, each output line shows the time it was presented, and that Step's finish time, duration, and Exit Code appear with that Step.
3. **Given** a successful Pipeline Run, **When** the log finishes, **Then** a closing summary reports the status succeeded, the number of Steps that ran, the duration of the run, and the time the run finished.
4. **Given** a successful Step whose command writes nothing, **When** the developer runs it, **Then** the Step still appears with its id, start time, finish time, duration, and Exit Code, and empty output is not reported as a failure.
5. **Given** a Pipeline Run whose Steps start one after another, **When** the developer reads the timestamps, **Then** the setup time is at or before the first Step's start, each Step's start is at or before its finish, and the next Step starts at or after the previous Step's finish.

---

### User Story 2 - Find the latest run afterward (Priority: P2)

A developer finishes a run and later opens the latest run log from the working directory. The saved text is the same run log they read while the pipeline executed. The next completed run replaces that log. The saved log is a local record, not part of the Pipeline Definition.

**Why this priority**: The live log scrolls away. The developer needs one place to open the same account of the latest run after the terminal session moves on.

**Independent Test**: Complete a run, open the latest run log in the working directory, and confirm it matches the log just shown. Complete a second run and confirm the stored log is only the second run.

**Acceptance Scenarios**:

1. **Given** a Pipeline Run that started and then finished, successfully or not, **When** the developer opens the latest run log in the working directory, **Then** the saved text is the run log shown for that run.
2. **Given** an existing latest run log, **When** a later Pipeline Run finishes, **Then** the stored log is replaced by the later run's log.
3. **Given** a Pipeline Definition file in another directory, **When** the developer runs that path, **Then** the latest run log is still written in the invoker's working directory, not beside the definition file.

---

### User Story 3 - Keep a record only when a run starts (Priority: P3)

A developer points the CLI at a definition that cannot be run, or runs a pipeline whose step fails. A rejected definition produces an error, starts no command, and leaves any existing latest run log unchanged. A run that fails after it starts keeps the log through the failed step, names that step and its Exit Code or start error, omits later steps, and still saves that log.

**Why this priority**: A missing or stale log is misleading. The developer must be able to tell a run that never started from a run that failed, and must still be able to inspect the failure afterward.

**Independent Test**: Run an invalid definition while a latest run log already exists, and confirm the error appears, no step runs, and the stored log is unchanged. Then run a three-step pipeline whose second command fails, and confirm the stored log includes the first step, the failed second step with its Exit Code and output, and no third step.

**Acceptance Scenarios**:

1. **Given** a path that cannot be read, YAML that cannot be parsed, or a definition that fails validation, **When** the developer runs it, **Then** TRUSS reports a user-visible error, starts no Step, writes no new run log, and leaves an existing latest run log unchanged.
2. **Given** a valid Pipeline Definition whose second Step exits non-zero, **When** the developer runs it, **Then** the run log includes the successful first Step and the failed second Step with its output, start time, finish time, duration, and Exit Code, names the pipeline and the failed Step, omits the third Step, and that same log is saved.
3. **Given** a valid Pipeline Definition whose command cannot be started, **When** the developer runs it, **Then** that Step is shown as failed with a start time, a finish time, a duration, a diagnostic, and no Exit Code, later Steps do not run, and the log is saved.
4. **Given** the latest run log cannot be created in the working directory, **When** the developer runs a valid definition, **Then** TRUSS reports a user-visible error, starts no Step, and leaves any existing latest run log unchanged.

---

### User Story 4 - Ask for help or the version (Priority: P4)

A developer opens a terminal and asks what the CLI can do, or which version is installed, without intending to run a pipeline. Help names the commands. The version command prints the product version. Neither command starts a Step.

**Why this priority**: The run log is useful only after the developer can discover `truss run`. Help and version are part of the same local workflow and must not execute a pipeline.

**Independent Test**: Invoke help and version with no pipeline file present. Confirm each prints its information, exits 0, and does not create a run log. Invoke an unknown command and confirm usage is printed, the process exits non-zero, and no Step runs.

**Acceptance Scenarios**:

1. **Given** any working directory, **When** the developer invokes `truss`, `truss help`, or `truss --help`, **Then** TRUSS prints usage that names `run`, `help`, and `version`, exits 0, and does not start a Step or write a run log.
2. **Given** any working directory, **When** the developer invokes `truss version` or `truss --version`, **Then** TRUSS prints the product version, exits 0, and does not start a Step or write a run log.
3. **Given** any working directory, **When** the developer invokes anything else, **Then** TRUSS prints usage, exits non-zero, and does not start a Step or write a run log.

---

### Edge Cases

- A Step command succeeds with empty output. The Step still has a start time, a finish time, and a duration.
- Two occurrences fall in the same second. Their timestamps may match, and the log order remains the order they happened.
- A Step writes more than one line. Each presented line carries the time TRUSS showed it.
- A Step command writes output and then exits non-zero.
- A Step command cannot be started.
- The pipeline path does not exist, is not a file, or cannot be read.
- The file is empty or is not valid YAML.
- The definition fails validation before any Step starts.
- `truss run` is invoked with no path and `truss.yaml` is missing or unreadable. *Changed by [spec 016](../016-project-layout/spec.md): Bare `truss run` exits 1 and lists the flows in `.truss/flows/`.*
- `truss run` is given a definition outside the working directory. The latest run log still belongs to the working directory.
- A latest run log already exists when a definition is rejected.
- A latest run log already exists when a later run finishes.
- `.truss` in the working directory exists and is not a directory, or the latest run log cannot be written.
- `.truss`, `.truss/data` or `.truss/data/cache` is a symbolic link, or `.truss` holds a link where TRUSS writes the run log (a checkout can carry one): TRUSS refuses the folder link before any Step starts and never writes into a link's target (FR-018). *Changed by [spec 016](../016-project-layout/spec.md): The folders are `.truss/state/<flow>/`, `.truss/state/<flow>/data` and `.truss/state/<flow>/data/cache`. A planted link at `.truss`, `.truss/flows`, `.truss/state` or `.truss/state/<flow>` is refused, and a link (or any other existing file) at `.truss/.gitignore` is never written through; the run goes on (spec 016 FR-014, FR-015 and SC-003).*
- The working directory cannot be determined, so the environment is not ready.
- Help or version is invoked with extra arguments.
- `truss run --help` is an explicit path, consistent with `truss run [<path>]`, and is not the help command. *Changed by [spec 016](../016-project-layout/spec.md): The argument is a path or a flow name (spec 016 FR-009), so `truss run --help` is read as a flow name. Whether it should print help is open.*
- The same workflow is invoked on Linux, macOS, and Windows. The host shell and operating system values differ by host. The facts in the setup section are the same kinds of facts.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When a Pipeline Run starts, the run log MUST begin with a setup section before any Step output. The setup section MUST state that the environment was captured and is ready, MUST show the operating system, architecture, host shell, and working directory the Steps will use, and MUST show the time that occurrence happened.
- **FR-002**: The setup section MUST NOT list environment variable names, environment variable values, or secrets. TRUSS MUST NOT add a secret store, secret declaration, or a dump of the invoker's environment in this feature.
- **FR-003**: For every Step that runs a command, the run log MUST show that Step's id, the time the Step started, the command output as produced, the time each output line was presented, the time the Step finished, the Step's duration, and the command's Exit Code. Output MUST appear as it is produced. Empty output MUST remain empty and MUST NOT be treated as a failure.
- **FR-004**: A Step that cannot be started MUST appear in the run log as failed, with a start time, a finish time, a duration, a diagnostic, and no Exit Code.
- **FR-005**: After the last started Step, the run log MUST close with a summary that reports the Pipeline Run status, the number of Steps that started, the duration of the run, and the time the run finished.
- **FR-006**: When a Step fails, the run log MUST include the Steps that started, MUST name the pipeline and the failed Step together with that Step's Exit Code or start error, and MUST omit later Steps. Those later Steps MUST NOT appear as succeeded.
- **FR-007**: The text saved as the latest run log MUST be the same text as the run log shown for that run, including setup, started Steps, and summary.
- **FR-008**: TRUSS MUST write that latest run log at `.truss/run.log` in the invoker's working directory. A finished run MUST replace the previous latest run log. This feature MUST NOT keep older runs, and MUST NOT edit ignore files. *Changed by [spec 016](../016-project-layout/spec.md): The log is `.truss/state/<flow>/run.log`. The exception is `.truss/.gitignore`, which spec 016 FR-015 creates once and never changes.*
- **FR-009**: If the Pipeline Definition cannot be read, parsed, or validated, TRUSS MUST report a user-visible error, MUST NOT start a Step, MUST NOT write a new run log, and MUST leave an existing `.truss/run.log` unchanged. *Changed by [spec 016](../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log` (spec 016 FR-013).*
- **FR-010**: If the environment is not ready or `.truss/run.log` cannot be created before Steps start, TRUSS MUST report a user-visible error, MUST NOT start a Step, and MUST leave an existing latest run log unchanged. *Changed by [spec 016](../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log` (spec 016 FR-013).*
- **FR-011**: A Pipeline Run that starts and then fails MUST still save its run log.
- **FR-012**: `truss`, `truss help`, and `truss --help` MUST print usage that names `run`, `help`, and `version`, MUST exit 0, and MUST NOT start a Step or write a run log.
- **FR-013**: `truss version` and `truss --version` MUST print the product version, MUST exit 0, and MUST NOT start a Step or write a run log.
- **FR-014**: Any other invocation MUST print usage, MUST exit non-zero, and MUST NOT start a Step or write a run log. *Changed by [spec 025](../025-cli-tidy/spec.md): an unknown command names itself with a suggestion and the short usage line; wrong arguments print that command's usage.*
- **FR-015**: `truss run` MUST keep the current load and execution behavior: no path loads `truss.yaml` from the invoker's working directory; a path loads that file; Steps run in definition order on the host shell; the first failed Step fails the Pipeline Run and later Steps do not run. The process exits 0 when the Pipeline Run succeeded and the latest run log was saved. The process exits non-zero when the definition was rejected, the environment or log location was not usable, or the Pipeline Run failed. *Changed by [spec 016](../016-project-layout/spec.md): Bare `truss run` exits 1 and lists the flows in `.truss/flows/` (spec 016 FR-010).*
- **FR-016**: This feature MUST NOT add Pipeline Definition fields. Schema version `"1"` remains version, pipeline name, and Steps with `id` and `run`. Unknown fields remain ignored.
- **FR-017**: Every occurrence TRUSS writes in the run log MUST show the local date and time it happened. Occurrences are the setup section, each Step start, each presented line of Step output, each Step finish, and the closing summary. A timestamp MUST NOT be earlier than the timestamp of the previous occurrence. The command's own text MUST remain that command's output. A timestamp does not replace duration: each started Step and the closing summary MUST still show duration.
- **FR-018**: TRUSS MUST NOT write through a symbolic link (or Windows junction) in its own folders. If `.truss`, `.truss/data` or `.truss/data/cache` in the working directory is a link, TRUSS MUST report an error naming it before any Step starts, MUST NOT start a Step, and MUST NOT write anything into the link's target. A link already inside `.truss` (for example at `.truss/run.log`) is never followed: its target is left unchanged, and after a finished run `.truss/run.log` is a regular file holding that run's log. The saved log keeps the usual permissions for a new file (0666 less the umask). (Added 2026-11 as a security fix: a checkout can carry such links, and a run log written into a folder the checkout chose, such as one a shell sources at login, can run code.) *Changed by [spec 016](../016-project-layout/spec.md): The folders are now `.truss/state/<flow>/`, `.truss/state/<flow>/data` and `.truss/state/<flow>/data/cache`, and the saved log is `.truss/state/<flow>/run.log`.*

### Quality, Safety & Compatibility *(mandatory when applicable)*

- **Testing**: Each user story MUST remain independently testable. Story 1 verifies the live success log. Story 2 verifies the saved copy matches that log and is replaced by the next finished run. Story 3 verifies rejection leaves the previous log unchanged and a failed run saves the failure. Story 4 verifies help and version do not start a run. Each story MUST be implemented test-first: a failing test for that observable outcome exists before the behavior is implemented.
- **Process execution**: This feature MUST NOT execute any command other than the Step `run` strings of a Pipeline Run that has started. Exit Codes MUST be preserved and shown. A failed Step MUST fail the Pipeline Run, and later Steps MUST NOT run. Cancellation, timeouts, and retries are out of scope. This feature does not define cancellation behavior.
- **Secrets**: The setup section and summary MUST NOT include environment variable names, environment variable values, or secret material. TRUSS MUST NOT invent a secret store or echo the invoker's environment. Output a Step command itself writes is part of that Step's log. TRUSS MUST NOT add credentials or environment dumps that the command did not write.
- **Cross-platform**: The latest run log path is `.truss/run.log` in the invoker's working directory on Linux, macOS, and Windows. *Changed by [spec 016](../016-project-layout/spec.md): The path is `.truss/state/<flow>/run.log`.* The setup section shows that host's operating system, architecture, host shell, and working directory. Step `run` strings remain host-shell strings. One Pipeline Definition is not required to be portable across host shells.
- **Compatibility**: This feature changes the unreleased run transcript from the host pipeline engine. A successful Step now includes start time, finish time, duration, and Exit Code. Every line TRUSS writes carries a timestamp. Every started run gains a setup section and a closing summary. `truss` with no command, help, and version exit 0 instead of a usage error. Load errors, validation errors, fail-fast execution, the Step Exit Code, and the process exit for a succeeded or failed Pipeline Run stay as they are. Schema version `"1"` is unchanged. No released contract is broken, because no version has been tagged. FR-018 (2026-11): a run in a working directory whose `.truss`, `.truss/data` or `.truss/data/cache` is a link now fails before any Step instead of writing into the link's target; the temporary log while a run is in progress has a random name (`.truss/run.log.<random>.tmp`) instead of `.truss/run.log.tmp`. The saved `.truss/run.log`, its text and its permissions are unchanged. *Changed by [spec 016](../016-project-layout/spec.md): The saved file is `.truss/state/<flow>/run.log` (spec 016 FR-013).*
- **Portability**: The run log describes the local machine where the developer invoked the CLI. This feature MUST NOT bind TRUSS to a CI provider, IDE, agent platform, or SDK language.

### Key Entities *(include if feature involves data)*

- **Run Log**: The text of one Pipeline Run: the setup section, each started Step Run, and the closing summary. Every occurrence in it carries the time it happened. It is what the developer reads while the run proceeds.
- **Environment Status**: The captured execution context shown before Steps: that the environment is ready, the operating system, the architecture, the host shell, and the working directory. It does not include variable names, variable values, or secrets.
- **Latest Run Log**: The single saved copy of the most recent Pipeline Run that started, stored at `.truss/run.log` in the invoker's working directory. *Changed by [spec 016](../016-project-layout/spec.md): The path is `.truss/state/<flow>/run.log`.* It is a local record, not part of the Pipeline Definition.
- **Step Run**: One started Step within the run log. It has an id, a start time, output as produced, a finish time, duration, and either an Exit Code or a start-failure diagnostic.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On every successful Pipeline Run, the setup section appears before step output, 100% of started Steps appear in definition order with their start time, output, finish time, duration, and Exit Code, 100% of TRUSS log lines show when that occurrence happened, and the summary reports success, that step count, the run's duration, and the time the run finished.
- **SC-002**: For 100% of Pipeline Runs that start and finish, the latest run log saved in the working directory is the same text the developer read for that run. The next finished run replaces it, and no older run remains stored by TRUSS.
- **SC-003**: For 100% of unreadable, unparseable, or invalid definitions, the developer sees an error, no Step command runs, and an existing latest run log is unchanged.
- **SC-004**: When a Step fails, the saved run log contains that Step's start time, output, finish time, duration, and Exit Code or start error, contains 0 later Steps, and names the failed Step.
- **SC-005**: Help and version complete without starting a Step or writing a run log. Usage names `run`, `help`, and `version`. An unknown invocation does not start a Step.
- **SC-006**: Across successful and failed runs, the setup section and summary contain none of the invoker's environment variable names or values and no secret list.

## Assumptions

- The actor is a developer running TRUSS on their own machine. The host pipeline engine already loads, validates, and runs a schema version `"1"` pipeline sequentially on the host shell.
- The latest run log file is `.truss/run.log`. *Changed by [spec 016](../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`. The one ignore file TRUSS creates is `.truss/.gitignore` (spec 016 FR-015).* One file holds the latest run. TRUSS does not create a history directory and does not modify ignore files. The developer may leave `.truss/` uncommitted.
- The working directory in the setup section is the invoker's working directory, shown so the developer can confirm where Step commands execute. An explicit pipeline path does not move the log.
- Duration is how long a Step or the Pipeline Run took. It stays in the log beside the timestamps. It is shown in seconds, including a fractional part when the Step or run finishes in under a minute. The same text is saved.
- A timestamp is the machine's local date and time, shown as `YYYY-MM-DD HH:MM:SS`. It records when an occurrence happened. Two occurrences in the same second may show the same timestamp. The log order is still the order they happened.
- The product version is printed as one line. Help prints a short usage text that includes `run`, `help`, and `version`, and that `run` accepts an optional path.
- `truss run --help` is still a path argument to `run`. Only a bare `truss`, `truss help`, and `truss --help` are the help command.
- A Step that cannot be started still has a start time, a finish time, and a duration, because the attempt finished, and it has no Exit Code.
- The summary counts Steps that started, including a Step that failed to start. Steps omitted after a failure are not counted as ran.
- If saving the log is impossible, TRUSS discovers that before starting Steps, so a started run can always leave a saved log when it finishes.
- A run that is killed outright (power loss, SIGKILL) can leave its temporary log, `.truss/run.log.<random>.tmp`, behind. *Changed by [spec 016](../016-project-layout/spec.md): The temporary file is in `.truss/state/<flow>/`.* TRUSS does not clean these up; they can be deleted by hand.
- Output written by a Step command is shown even when that command prints sensitive values. TRUSS does not detect or mask those values in this feature, and it does not add any of its own.
- Status values shown for a finished run remain `succeeded` and `failed`.
- Glossary terms in `docs/GLOSSARY.md` are the vocabulary for this specification.
- Out of scope: dependency graphs, `needs` or `requires`, concurrency, cancellation status, per-step environment or working directory, timeouts, retries, secret declaration, container or remote execution, a daemon, an API, an SDK, matrix builds, cache, artifacts, and CI-provider environments.
