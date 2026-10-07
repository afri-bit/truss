<!-- Title: Conventional Commit, e.g. "feat(pipeline): validate duplicate step ids" -->

## Summary

<!-- What changes and why, in a few sentences. -->

## Spec and tasks

<!-- Required for behavior changes. Mechanical changes: write "n/a (mechanical)". -->

- Spec: `specs/NNN-short-name/` <!-- status must be approved or implementing -->
- Tasks: T000, T000
- Acceptance criteria covered: FR-000, FR-000
- ADR: <!-- docs/adr/NNNN-…md if a public contract or boundary changes, else "none" -->

## Checklist

- [ ] The linked spec is `approved` (or `implementing`); no behavior beyond it
- [ ] Task checkboxes ticked; spec status and `specs/README.md` updated
- [ ] Tests added or updated and cite the FR IDs they cover
- [ ] `make fmt lint test` passes locally (`make test-integration` if executors changed)
- [ ] Import boundaries respected; the engine emits typed events only
- [ ] Cross-platform: `filepath`, no shell assumptions, CRLF-safe
- [ ] Public contract (schema, CLI, `/v1` API) unchanged, **or** spec + ADR linked
- [ ] User docs updated (`docs/user/`, `examples/`) if user-visible
- [ ] `CHANGELOG.md` Unreleased updated if user-visible

## Notes for reviewers

<!-- Risky areas, follow-ups, deviations from plan.md. -->
