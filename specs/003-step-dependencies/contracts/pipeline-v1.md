# Contract: Pipeline Definition v1

**Status**: Extends `specs/001-host-pipeline-engine/contracts/pipeline-v1.md`  
**Compatibility**: Schema version stays `"1"`. `needs` is optional. A definition that does not contain it stays valid and keeps the execution contract in `specs/001-host-pipeline-engine/contracts/host-execution.md`. No version has been tagged. Unknown fields stay ignored, so `depends_on` does not declare a Dependency.

## Document

```yaml
version: "1"

pipeline:
  name: build

steps:
  - id: test
    needs: [build]
    run: go test ./...
  - id: build
    run: go test ./...
```

`needs` may be omitted. `needs: []` means the same as omitting it.

## Acceptance

Everything accepted by the v1 contract, plus:

- `needs`, when present, is a list.
- Each entry is a string Step id. It is trimmed the same way `id` is trimmed.
- Each entry names a different Step in this file.
- No list repeats an id.
- The declared Dependencies contain no cycle.

## Rejection

Reject before any Step command starts, with no new run log, when any v1 rejection applies, and when:

| Fault | Diagnostic |
|---|---|
| `needs` is not a list of string Step ids | `step "<id>" needs must be a list of step ids` |
| An entry names no Step in this file | `step "<id>" needs unknown step "<entry>"` |
| An entry is this Step's id | `step "<id>" depends on itself` |
| A list repeats an id | `step "<id>" repeats step "<entry>"` |
| Dependencies form a cycle | `dependency cycle: <id>, <id>, ...` |

The cycle names every Step in that cycle once. It starts with the member written earliest in the file, then follows `needs`.

Existing structural errors are unchanged and are reported before a `needs` error. The CLI prints the diagnostic as `error: <diagnostic>` on standard error, as in `specs/002-local-run-log/contracts/cli.md`. An existing `.truss/run.log` is unchanged.

## Filename

Unchanged. The document may use any filename. `truss.yaml` is only the default lookup when `truss run` is given no path.
