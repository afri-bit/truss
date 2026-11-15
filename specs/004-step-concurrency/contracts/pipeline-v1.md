# Contract: Pipeline Definition v1

**Status**: Extends `specs/003-step-dependencies/contracts/pipeline-v1.md`  
**Compatibility**: Schema version stays `"1"`. `pipeline.concurrency` is optional. A definition that uses only recognised fields and omits it stays valid and runs as before. **Tightening**: a definition that contains an unrecognised field was accepted before and is now rejected. This replaces the rule that unknown fields are ignored. No version has been tagged.

## Document

```yaml
version: "1"

pipeline:
  name: build
  concurrency: 2

steps:
  - id: lint
    run: golangci-lint run ./...
  - id: test
    run: go test ./...
  - id: package
    needs: [lint, test]
    run: go build ./cmd/truss
```

## Recognised fields

| Where | Fields |
|---|---|
| Top level | `version`, `pipeline`, `steps` |
| `pipeline` | `name`, `concurrency` |
| Each Step | `id`, `run`, `needs` |

## `concurrency`

Optional. The largest number of Steps that may run at the same time. Omitted means 1. A value of 1 behaves exactly as omitted.

Accepted: a decimal whole number of at least 1 (`1`, `2`, `16`). A number too large for the host integer means "as many as are ready".

## Rejection

Reject before any Step command starts, with no new run log, when any earlier rejection applies, and when:

| Fault | Diagnostic |
|---|---|
| `concurrency` is `0`, negative, a fraction (`1.5`, `1.0`), text (`"two"`, `"3"`), `true`, `0x10`, `0o7`, `1_000`, `+3`, empty, or null | `pipeline concurrency must be a whole number of at least 1, got "<value>"` |
| `concurrency` is a list or a mapping | `pipeline concurrency must be a whole number of at least 1, got a list` / `got a mapping` |
| An unrecognised field at the top level | `unknown field "<name>" at top level` |
| An unrecognised field under `pipeline` | `unknown field "<name>" in pipeline` |
| An unrecognised field in a Step | `unknown field "<name>" in step "<id>"`, or `in step <N>` (1-based) when the Step has no id |

Check order: schema version, unknown field, name, `concurrency`, steps, `id`, `run`, then `needs` (spec 003 order). Existing diagnostics are unchanged. The CLI prints `error: <diagnostic>` on standard error. An existing `.truss/run.log` is unchanged.

## Filename

Unchanged. The document may use any filename. `truss.yaml` is only the default lookup when `truss run` is given no path.
