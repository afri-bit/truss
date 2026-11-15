# Contract: Pipeline Definition v1

**Status**: New public contract  
**Compatibility**: First release. A later incompatible change to this shape must identify compatibility impact before implementation.

## Document

```yaml
version: "1"

pipeline:
  name: hello-world

steps:
  - id: hello
    run: echo "Hello from TRUSS"
```

`steps` is a sibling of `pipeline`, not nested inside it.

## Acceptance

- `version` is the string `"1"`.
- `pipeline.name` is a non-empty string. *Changed by [spec 016](../../016-project-layout/spec.md): `pipeline.name` is optional and defaults to the file stem.*
- `steps` has at least one element.
- Each `id` is a non-empty string and unique in the file.
- Each `run` is a non-empty string.
- The file is valid YAML and readable.

## Rejection

Reject before any Step command starts when:

- the path is missing, not a file, or unreadable
- the document is empty or not valid YAML
- `version` is missing or not `"1"`
- the name is missing or empty *Changed by [spec 016](../../016-project-layout/spec.md): a missing name is not rejected; it defaults to the file stem.*
- `steps` is missing, empty, or not a list
- an `id` or `run` is missing or empty
- two Steps share an `id`

The error names the problem. No Step command runs.

## Filename

The document may use any filename. `truss.yaml` is only the default lookup when `truss run` is given no path. *Changed by [spec 016](../../016-project-layout/spec.md): `truss.yaml` is project metadata and is never a pipeline file. Bare `truss run` exits 1.*
