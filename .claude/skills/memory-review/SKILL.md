---
name: "memory-review"
description: "Review the shared agent memory for stale, duplicate or conflicting entries; supersede or archive them and rebuild the index. Use periodically, before a release, or when /recall finds conflicts."
argument-hint: "Optional topic to focus on"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Memory review

Delegate to the `memory-keeper` agent (haiku) with the input as focus.

1. Run `python3 scripts/memory.py validate` and `python3 scripts/memory.py stale`. Fix validation failures first.
2. For each entry past `review_by`, and each entry touching the focus topic: check it against the current
   code, the constitution, specs and ADRs. Keep it (move `review_by` forward with `update`), correct it,
   supersede it (`new ... --supersedes MEM-nnnn`) or archive it (`update ... --status archived`).
3. Find duplicates (same topic, same fact) and merge them by superseding the weaker entry. Flag conflicts
   with the constitution, a spec or an ADR to the maintainer instead of resolving them in memory.
4. Remove nothing: superseded and archived entries stay as history. Never edit past log lines.
5. `python3 scripts/memory.py index`, `python3 scripts/memory.py validate`, then log: `log --actor memory-keeper --event memory_updated ...`.
6. Report what was kept, changed, superseded and archived, plus any "Needs human" items. Do not commit.
