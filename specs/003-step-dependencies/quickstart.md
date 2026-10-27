# Quickstart: Step Dependencies

Manual check after the feature is implemented. Commands assume the `truss` binary is available and the working directory is a scratch project. Step commands below are host-shell strings.

## 1. Run a definition that does not use needs

Create `truss.yaml`:

```yaml
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo "Hello from TRUSS"
  - id: second
    run: echo second
```

```text
truss run
```

Expect exit 0. `hello` appears before `second`. The setup section, timestamps, durations, and `summary succeeded steps 2` match the local run log. `.truss/run.log` matches the text just printed.

## 2. Honor needs

Replace the file with:

```yaml
version: "1"

pipeline:
  name: build

steps:
  - id: test
    needs: [build]
    run: echo test
  - id: build
    run: echo build
  - id: scan
    run: echo scan
```

```text
truss run
```

Expect exit 0 and this start order: `build`, `test`, `scan`. Each step shows `exit 0`. The log has no `needs` line.

## 3. Reject a cycle

Use a definition where `a` needs `b` and `b` needs `a`. Run it while `.truss/run.log` already exists.

Expect `error: dependency cycle: a, b` on standard error when `a` is written first, exit 1, no step line, and the same `.truss/run.log` as before.

Repeat once each for an unknown id, a Step that needs itself, a repeated id, and `needs` set to a string. Each error names that Step, and no command runs.

## 4. Stop the whole run

```yaml
version: "1"

pipeline:
  name: build

steps:
  - id: build
    run: exit 1
  - id: test
    needs: [build]
    run: echo test
  - id: lint
    run: echo lint
```

Expect exit 1. The log contains `build` with a non-zero exit, `summary failed steps 1`, and an error line naming `build`. It does not contain `test` or `lint`. `.truss/run.log` matches that log.
