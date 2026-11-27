# Variables and secrets

Define a value once and use it everywhere; keep passwords and tokens out of your files.

## Define once

Put what every flow of the project needs in `truss.yaml`, and build on it in a flow:

```yaml
# truss.yaml
version: "1"
project:
  name: photos
env:
  DATA: ${HOME}/photo-data
```

```yaml
# .truss/flows/sort.yaml
env:
  INBOX: ${DATA}/inbox              # from the project
  PATH: ${TRUSS_PROJECT_DIR}/bin:${PATH}
steps:
  - id: sort
    dir: ${INBOX}
    run: sort-photos
```

`env` can also be set on a job or a step; the nearest wins. `${NAME}` is replaced by TRUSS before
the step starts, the same way on every system. `$$` is a literal `$`. See
[the order of the layers](how-a-run-works.md#the-environment-of-a-step).

## What TRUSS tells every step

| Variable | Value |
|---|---|
| `TRUSS_FLOW`, `TRUSS_STEP`, `TRUSS_JOB` | where the step is |
| `TRUSS_RUN_ID`, `TRUSS_TRIGGER`, `TRUSS_ATTEMPT` | which run and why |
| `TRUSS_PROJECT_DIR` | the project root |
| `TRUSS_TEMP_DIR` | a folder for this run only |
| `TRUSS_DATA_DIR`, `TRUSS_CACHE_DIR` | folders that stay between runs |
| `TRUSS_OS`, `TRUSS_ARCH` | the system |

The [reference](pipeline-reference.md#built-in-variables) has them all. A file cannot set them.

## Secrets

```console
$ truss secret set NPM_TOKEN          # asked twice, never echoed
$ truss secret list                   # names only
```

```yaml
steps:
  - id: publish
    secrets: [NPM_TOKEN]              # only this step gets it, as $NPM_TOKEN
    run: npm publish
```

- A stored secret's value is replaced by `***` in every log.
- Only the steps that list a secret get it. `${NPM_TOKEN}` in `env` does not read the store.
- The store is a file only you can read (`~/.truss/secrets.json`). It is not encrypted.

## Passing values between steps

Each step is its own process: a variable one step `export`s does not reach the next. Today, share
values through `env`, or through a file in `$TRUSS_TEMP_DIR` (one run) or `$TRUSS_DATA_DIR` (across
runs):

```yaml
steps:
  - id: version
    run: git describe --tags > "$TRUSS_TEMP_DIR/version"
  - id: build
    needs: [version]
    run: go build -ldflags "-X main.v=$(cat "$TRUSS_TEMP_DIR/version")" ./...
```

## See what a step gets

```console
$ truss flows show sort sort          # every variable, where it comes from, set or missing
$ truss run sort --step sort --env DATA=/tmp/test-data
```
