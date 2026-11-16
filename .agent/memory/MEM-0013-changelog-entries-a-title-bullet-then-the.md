---
id: MEM-0013
type: semantic
title: "Changelog entries: a title, a hard line break, then the explanation"
summary: Every CHANGELOG bullet is a short title ending in two spaces, then the explanation on the next line.
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

Every entry in `CHANGELOG.md` (Added, Changed and Fixed alike; set by the maintainer on 2026-11-17) is a bullet with a short title, then the explanation on the next line, indented two spaces. The title line ends with two trailing spaces, which is a Markdown hard line break:

```
- TRUSS Desktop: toggle between cards and a list on the Flows page␠␠
  The longer description.
```

(`␠␠` stands for the two spaces; write real spaces.)

## Why it matters

The maintainer wants entries scannable by title. Without the two trailing spaces, Markdown joins the description onto the title line. Editors that trim whitespace break it, so check `git diff` keeps them.

## How to apply

Use this shape for every new entry under `[Unreleased]`, and keep the file LF-only (`.gitattributes`) and the Keep a Changelog headings the release check accepts. The released `[0.1.0]` section keeps its original one-line entries.
