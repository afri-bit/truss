# Feature Specification: User documentation with an overview

**Spec**: `032-user-docs` | **Created**: 2026-11-27 | **Status**: Implemented

Extends [`017-docs-site`](../017-docs-site/spec.md). Risk class: tier 1 (documentation and the
docs site; no product behaviour). Scope set by the maintainer on 2026-11-25 (milestone v0.3.0):
"make documentation structured and easy to read for users; product and end users oriented, with
details and good structure".

## Why

The docs were complete but grouped by component (reference pages), so a new user had to read
several long pages to do one thing, and nothing explained how a run works as a whole.

## Requirements

- **FR-001** An overview page says what TRUSS is, when to use it and when something else fits
  (CI, act, Make/Task, cron), the words TRUSS uses, and where to read next.
- **FR-002** Task guides, each one page that opens with what it is for and a working example, then
  the details, then links to the reference: schedules, hooks, notifications, variables and secrets,
  the command line (existing), Windows/macOS/Linux.
- **FR-003** A concepts page, "How a run works": processes (the run inside the truss process, each
  step a child in its own process tree), order and failure, stopping, the layers of a step's
  environment, where things are kept, local versus daemon runs.
- **FR-004** A troubleshooting page: symptom, cause, fix, and where to look.
- **FR-005** The site sidebar has the sections Start here, Guides, Concepts, Reference, Help, For
  contributors; the README's docs table follows the same order. The site's "Docs" link opens the
  overview.
- **FR-006** Guides link to the reference instead of repeating it (one source of truth per fact).
  A strict site build (`SYNC_STRICT=1`) finds no broken link.

## Acceptance

`npm test` and `SYNC_STRICT=1 npm run build` in `site/` pass with the 16 pages.
