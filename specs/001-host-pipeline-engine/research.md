# Research: Host-Executed Pipeline Engine

## YAML decoder

- **Decision**: Decode Pipeline Definitions with `gopkg.in/yaml.v3`.
- **Rationale**: The Go standard library has no YAML decoder. This milestone's public definition format is YAML. One decoder dependency covers that concrete need.
- **Alternatives considered**: A hand-written subset parser (rejected: easy to drift from YAML). `gopkg.in/yaml.v2` (rejected: v3 is the maintained module that preserves string-vs-number distinctions needed for schema version `"1"`).

## Schema version type

- **Decision**: `version` must be the YAML string `"1"`. A number, a boolean, or any other string is an unsupported schema version and fails validation before execution.
- **Rationale**: The spec example and FR-006 require schema version `"1"`.
- **Alternatives considered**: Accept integer `1` as an alias (rejected: it widens the public contract beyond the specified string).

## Unknown YAML fields

- **Decision**: Ignore unknown fields. Still reject every missing or empty required field listed in FR-005.
- **Rationale**: The spec names the required checks and forbids adding optional definition fields. It does not require a closed schema. A typo of a required field still fails validation because the required value is missing.
- **Alternatives considered**: Reject unknown fields (rejected: extra behavior not required for this milestone).

## Host shell

- **Decision**: Each Step `run` string is one argument to the host shell, never an argv list.
  - Unix-like (`darwin`, `linux`, other non-Windows): `/bin/sh -c <run>`
  - Windows: `%COMSPEC%` when set, otherwise `cmd.exe`, invoked as `cmd /c <run>`
- **Rationale**: FR-009 requires the host shell. `/bin/sh` is the POSIX shell on Unix-like hosts and does not bind TRUSS to bash. Windows has no `/bin/sh`; `cmd.exe` is that host's shell. The spec already says one Pipeline Definition is not required to be portable across different host shells.
- **Alternatives considered**: Always bash (rejected: binds Unix hosts to bash). Argument-list execution (rejected in clarification). A user-configured shell (rejected: no such field in this milestone).

## Working directory and environment

- **Decision**: Capture the process working directory and environment once when the run starts. Every Step is a new shell process started in that directory with that environment.
- **Rationale**: FR-010 says each Step Run inherits the invoker, not the previous Step. A `cd` or `export` inside one `run` string does not change the next Step, because the next Step is a new process.
- **Alternatives considered**: One long-lived shell for the whole Pipeline (rejected: later Steps would inherit earlier shell state, which FR-010 does not say). Per-Step directory or env fields (rejected by FR-006).

## Output and Exit Code

- **Decision**: Store stdout and stderr separately on the Step Run. Both are shown in the CLI report. The Exit Code is the process exit code. Empty stdout is success when the Exit Code is 0. A command that cannot be started has status `failed`, no Exit Code, and a diagnostic error; stderr may be empty.
- **Rationale**: FR-011 and FR-014 require output, Exit Code, and failure context. Splitting the streams keeps the Exit Code as the success signal and still shows shell errors that were written to stderr.
- **Alternatives considered**: Drop stderr (rejected: start and shell failures often write only there). Merge into one buffer (rejected: interleaving is unnecessary for this milestone).

## CLI process Exit Code

- **Decision**: The `truss` process exits 0 when the Pipeline Run succeeded. It exits 1 when the file cannot be loaded, YAML cannot be parsed, validation fails, or the Pipeline Run failed. The Step Exit Code is reported in the Result and is not reused as the process Exit Code.
- **Rationale**: The spec assumption requires a non-zero process status for those failures and a human-readable Result that still carries the Step Exit Code.
- **Alternatives considered**: Exit with the failed Step's code (rejected: load and validation failures have no Step Exit Code, and the assumption does not require the codes to match).

## Cancellation

- **Decision**: No cancellation, timeout, retry, or signal feature. Blocking engine and executor calls take `context.Context` first and do not store it. The CLI passes a background context. Subprocess creation uses that context so a canceled context cannot leave the call blocked, but this milestone does not define a `cancelled` status.
- **Rationale**: The spec puts cancellation out of scope. The constitution still requires context on process execution.
- **Alternatives considered**: Ctrl+C mapped to status `cancelled` (rejected: that status is out of scope).

## Package boundaries

- **Decision**: `internal/pipeline` parses, validates, and plans. `internal/engine` executes a plan through `internal/executor`. `cmd/truss` only resolves the path, calls the engine, prints the Result, and sets the process Exit Code.
- **Rationale**: FR-015 and FR-016 require those boundaries to be independently testable. Extra parser, validator, daemon, and remote-executor packages are not needed for this milestone.
- **Alternatives considered**: The finer README sketch (`parser/`, `validation/`, `executor/host`) (rejected: those splits do not add a second implementation or a separate test boundary beyond package-level tests).

## Public contracts introduced

- **Decision**: YAML schema v1, host-shell sequential fail-fast semantics, and `truss run` are new public contracts. Later incompatible changes must call out compatibility impact before implementation.
- **Rationale**: The spec names these as versioned interfaces. Nothing in the repository previously implemented them.
- **Alternatives considered**: None. This is the first engine milestone.
