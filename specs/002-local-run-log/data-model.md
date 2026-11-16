# Data Model: Local Run Log

The pipeline file is unchanged. Schema version `"1"` is still `version`, `pipeline.name`, and `steps` with `id` and `run`. Unknown fields stay ignored. This model describes the run log, not a new definition format.

## Run Log

The text of one pipeline run, written live and saved as the same bytes.

| Field | Required | Rules |
|---|---|---|
| Lines | Yes | Every line TRUSS writes starts with a local timestamp `YYYY-MM-DD HH:MM:SS`, then one space, then the occurrence. A timestamp is never earlier than the previous one. |
| Setup | Yes | Five lines before any step output: environment ready, operating system, architecture, host shell, absolute working directory. No variable names, variable values, or secrets. |
| Steps | Yes, for each started step | Start line, zero or more output lines, finish line. Definition order. A step that does not start is absent. |
| Summary | Yes | One line after the last started step: status, the count of started steps, and the run duration. A failed run adds one error line naming the pipeline and the failed step. |

The run duration is the summary time minus the setup time. It is shown in seconds with three decimal places.

## Environment Status

Captured once, when the run starts, before steps.

| Field | Rules |
|---|---|
| Ready | The log says `environment ready` only when the working directory is known and the latest-log location can be written. |
| Operating system | `linux`, `macOS`, or `windows` on those hosts. Any other host uses its reported OS name. |
| Architecture | The host architecture name, such as `amd64` or `arm64`. |
| Shell | The host shell path that will execute step commands. |
| Directory | The absolute working directory of the invoker. An explicit pipeline path does not change it. |

## Step occurrence

One started step inside the run log.

| Field | Required | Rules |
|---|---|---|
| ID | Yes | The step `id` from the definition. |
| Start | Yes | Timestamped `started` line before that step's output. |
| Output | No | Each presented line of command output, including a blank line. Empty output adds no output line. The command text is unchanged after the occurrence prefix. |
| Finish | Yes | Timestamped `finished` line. Includes duration. Includes `exit <code>` when the command ran. Includes `error <diagnostic>` and no exit code when the command did not start. |
| Status | Yes, on the finish line | A zero exit finishes as a successful step. A non-zero exit or a start error finishes as a failed step. The pipeline summary carries `succeeded` or `failed`. |

Step duration is the finish time minus the start time, in the same seconds form as the run duration.

## Latest Run Log

| Field | Rules |
|---|---|
| Path | `.truss/run.log` in the invoker's working directory. *Changed by [spec 016](../016-project-layout/spec.md): The path is `.truss/state/<flow>/run.log`.* |
| Contents | The run log bytes of the most recent pipeline run that started and finished. |
| Replacement | The previous file is replaced only after that run finishes. A rejected definition does not create or replace it. |
| History | One file. Older runs are not stored. Ignore files are not edited. *Changed by [spec 016](../016-project-layout/spec.md): The one exception is `.truss/.gitignore` (FR-015).* |

## Relationships

- A run log has one environment status, one or more step occurrences when a run starts, and one summary.
- A latest run log is a saved run log. Help, version, usage errors, and rejected definitions do not have one.
- A failed step ends the step list. Later definition steps are not step occurrences and are not counted in the summary.

## State

| Situation | Latest run log |
|---|---|
| Definition cannot be read, parsed, or validated | Unchanged. No new file. |
| `.truss` or the temporary log cannot be created | Unchanged. No step starts. |
| Run finishes, succeeded or failed | Replaced with that run's log. |
| Final replace fails | Previous `.truss/run.log` stays. *Changed by [spec 016](../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.* The process exits non-zero. |
