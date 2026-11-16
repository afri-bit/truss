---
paths:
  - "docs/**"
  - "*.md"
  - ".agent/**"
---

# Documentation

| Where | What |
|---|---|
| `README.md` | Short product overview for users: what it is, install, quick start, pipeline definition |
| `CONTRIBUTING.md` | Contribution rules: checks, branches, commits, pull requests, releasing |
| `AGENTS.md` | Agent workflow only. Never duplicates the constitution |
| `CLAUDE.md` | How Claude Code works in this repo. Imports `AGENTS.md` |
| `docs/MILESTONE.md` | Release scope |
| `docs/GLOSSARY.md` | Terms. Use them exactly; do not add synonyms |
| `docs/agents.md` | The agent setup: roster, flow, gates, hooks |
| `docs/decisions/` | `NNNN-title.md` decision records (ADRs) |
| `CHANGELOG.md` | Keep a Changelog, `[Unreleased]` until a tag |
| `specs/` | Spec Kit artifacts, generated only by `/speckit-*` |

- Plain language, short sentences, active voice. Lead with the point. Write for a reviewer on a phone.
- Link, do not copy: one source of truth per fact.
- A decision record is half a page: Status, Date, Context, Decision, Consequences. Status is
  `Proposed` until the maintainer approves the PR.
- Update docs in the same PR as the behavior they describe.
- No secrets, tokens or personal data in docs or examples.
