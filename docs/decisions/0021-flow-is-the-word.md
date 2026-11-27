# 0021. "Flow" is the word

- Status: Proposed
- Date: 2026-11-25

## Context

The flow file's main block was `pipeline:`, the folder `.truss/flows/`, Desktop said "Flows", and
the CLI had both `truss flows` and `truss pipelines`. The glossary had to explain that a flow is a
pipeline. The maintainer chose "flow" for v0.3.0 (milestone, 2026-11-25). The file schema, the CLI
and the API are public contracts (constitution V).

## Decision

1. "Flow" is the user-facing word in files, help, output, Desktop and docs.
2. The file accepts `flow:`; `pipeline:` stays accepted with the same meaning. Both in one file is
   an error. The schema `version` stays `"1"`: the change only adds a name.
3. Released machine contracts keep "pipeline": the API paths and JSON fields, `TRUSS_PIPELINE`,
   JSON values such as the variable source `pipeline`, the `truss pipelines` command and the home
   folder `~/.truss/pipelines/`. A new major API version may rename them; none is planned.
4. Human-readable output may change wording in v0.x; the changelog says so.

## Consequences

- One word for users; old files, scripts on JSON, and API clients keep working.
- Scripts that match English output must update; the changelog lists the changed lines.
- Code keeps the Go type `pipeline.Pipeline`; renaming internals is not worth the churn.
