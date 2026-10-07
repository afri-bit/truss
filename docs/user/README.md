# User Guide

The user guide grows with each release. TRUSS is pre-alpha, and no
user-facing features are released yet.

Planned pages for v0.1.0:

| Page | Covers | Spec |
|------|--------|------|
| Installation | Binaries, `go install`, verifying the version | — |
| Pipeline reference | Every field of the `version: 1` schema and validation errors | [001](../../specs/001-pipeline-schema-v1/spec.md) (draft) |
| CLI reference | `run`, `daemon`, `bind`, `status`, `logs`, `init` | not yet written |
| Running in CI | Using `truss run` from GitHub Actions and Jenkins | not yet written |
| Daemon and bindings | Workspaces, cron and manual triggers, run history | not yet written |

Until then, see the [README](../../README.md) and the illustrative
[example pipeline](../../examples/pipeline.yml).
