---
name: write-adr
description: Create a numbered Architecture Decision Record in docs/adr/ using the MADR template. Use when a change alters a public contract (pipeline schema, CLI surface, HTTP API /v1), adds or replaces a significant dependency, changes package boundaries or import rules, or when the user asks to record or document a decision.
---

# Write an ADR

## Steps

1. **Read** `docs/adr/README.md` and the ADRs related to the topic. If an
   accepted ADR already decides this, the new ADR must supersede it
   explicitly.
2. **Number**: highest `docs/adr/NNNN-*.md` + 1, four digits.
3. **Create** `docs/adr/NNNN-short-kebab-title.md` from
   `docs/adr/template.md`. Front matter: `status: proposed` (or `accepted`
   if the user states the decision is made), today's `date`,
   `decision-makers`.
4. **Fill**:
   - *Context and Problem Statement*: the forces, linked spec if any.
   - *Decision Drivers*: concrete and checkable (cross-platform, no CGO,
     contract stability, dependency cost …).
   - *Considered Options*: at least two real alternatives.
   - *Decision Outcome* with *Consequences* (good **and** bad) and
     *Confirmation* (the lint rule, test, or review check that enforces it).
   - *Pros and Cons* per option.
5. **Supersede** if needed: set the old ADR's status to
   `superseded by ADR-NNNN` (the only edit allowed on an accepted ADR) and
   link both ways under *More Information*.
6. **Index**: add a row to the table in `docs/adr/README.md`.
7. **Link** the ADR from the relevant `plan.md` (*Public Contract Impact*
   or *Alternatives Considered*) and, if it changes import rules, update
   `docs/architecture.md` and `.golangci.yml` depguard rules together.
8. **Commit** as `docs(adr): NNNN short title`.

## Rules

- ADRs record decisions and rationale; requirements belong in specs.
- Never rewrite the substance of an accepted ADR.
