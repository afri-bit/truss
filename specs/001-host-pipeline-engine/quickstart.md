# Quickstart: Host-Executed Pipeline Engine

Manual check after the feature is implemented. Commands assume the `truss` binary is available and the working directory is a scratch project.

## 1. Run an explicit path

Create `pipeline.yaml`:

```yaml
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo "Hello from TRUSS"
```

```text
truss run pipeline.yaml
```

Expect process Exit Code 0, `status: succeeded`, and one Step `hello` with Exit Code 0 and the hello output.

## 2. Run the default file

Copy that definition to `truss.yaml` and run:

```text
truss run
```

Expect the same success report. A different filename still works when passed as a path. `truss run` with no path does not search for `pipeline.yaml`.

## 3. Reject an invalid file

Remove `version` from a copy and run it. Expect a validation error on standard error, process Exit Code 1, and no Step output.

## 4. Stop after a failure

```yaml
version: "1"

pipeline:
  name: fail-fast

steps:
  - id: first
    run: echo first
  - id: second
    run: exit 1
  - id: third
    run: echo third
```

On Windows, replace `exit 1` with `exit /b 1`.

Expect `first` succeeded, `second` failed with a non-zero Exit Code, `third` absent, Pipeline status `failed`, and process Exit Code 1.
