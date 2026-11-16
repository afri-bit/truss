# Quickstart: Local Run Log

Manual check after the feature is implemented. Commands assume the `truss` binary is available and the working directory is a scratch project.

## 1. Ask for help and the version

```text
truss help
truss version
```

Help prints usage that names `run`, `help`, and `version`, and exits 0. Version prints one version line and exits 0. Neither creates `.truss/run.log`.

## 2. Run the default file

Create `truss.yaml`:

```yaml
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo "Hello from TRUSS"
  - id: where
    run: echo "running in $PWD"
```

```text
truss run
```

Expect exit 0. The log starts with a heading line (`==== Setup: ... ====`, spec 020), then `setup environment ready`, `setup os`, `setup arch`, `setup shell`, and `setup directory` before any step output. Each of `hello` and `where` has `started`, a `|` line with that command's output, and `finished` with `duration` and `exit 0`. The log ends with `summary succeeded steps 2 duration`. Every line begins with a local timestamp `YYYY-MM-DD HH:MM:SS`. No environment variable names are listed.

Open `.truss/run.log`. It matches the text just printed.

## 3. Replace the saved log

Run `truss run` again. `.truss/run.log` is only the second run.

## 4. Reject an invalid file

Remove `version` from a copy and run that path while `.truss/run.log` already exists. Expect `error:` on standard error, exit 1, no step line, and the same `.truss/run.log` as before.

## 5. Stop after a failure

Run a definition whose second step exits non-zero and whose third step would print `later`. Expect the first step, the failed step with its exit code and duration, no `later`, a `summary failed` line, and an `error` line naming the pipeline and the failed step. Exit is 1. `.truss/run.log` contains that same log.
