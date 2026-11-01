---
name: "adr"
description: "Create a numbered decision record in docs/decisions. Use for a public contract change, a constitution amendment, a new dependency, a boundary or architecture change, or a change to the agent setup."
argument-hint: "Short title or the decision to record"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Decision record

1. List `docs/decisions/` to find the next number (zero-padded to four digits). If the directory or
   its `0000-template.md` is missing, create the template first (Status, Date, Context, Decision,
   Consequences). If the input is empty, ask for the decision in one sentence.
2. If the decision is not yet made, delegate to `architect` (opus) to weigh the options against the
   constitution and the milestone, and recommend one.
3. Delegate formatting to `docs-writer` (haiku) with the decided content and the file name
   `NNNN-kebab-title.md`.
4. Half a page: Status, Date, Context, Decision, Consequences (good, bad, follow-ups). Use glossary
   terms. Status is `Proposed` until the maintainer approves the PR.
5. If it supersedes another record, update the old status to `Superseded by NNNN` and link both ways.
6. Report the path and a two-line summary. Do not commit.
