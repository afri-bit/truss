---
id: MEM-0013
type: semantic
title: Changelog entries: a title bullet, then the explanation under it
summary: From 2026-11-17 each new CHANGELOG bullet is a short title with the detail as an indented paragraph below it.
topics: [changelog, owner-preferences, docs]
created: 2026-11-16T23:28:57Z
updated: 2026-11-16T23:28:57Z
author: orchestrator
source: maintainer instruction, 2026-11-17
status: active
confidence: high
related: [specs/021-flows-view-toggle]
---

## What

New `CHANGELOG.md` entries (from 2026-11-17) are a bullet holding a short title, then a blank line and the explanation as a paragraph indented two spaces under it:

```
- TRUSS Desktop: toggle between cards and a list on the Flows page

  The longer description.
```

## Why it matters

The maintainer wants entries scannable by title. A plain newline inside a list item renders joined to the title in Markdown, so the explanation goes in its own indented paragraph (no trailing-space hard breaks, which editors strip). Older entries keep their one-line form; do not rewrite them unasked.

## How to apply

Use this shape for every new entry under `[Unreleased]`. Keep the file LF-only (`.gitattributes`) and the Keep a Changelog headings the release check accepts.
