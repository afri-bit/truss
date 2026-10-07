---
name: write-spec
description: Create a new TRUSS feature specification (specs/NNN-short-name/spec.md) from the template, assign the next number, and add it to specs/README.md. Use when the user asks to spec, specify, or propose a new feature or behavior change, or when an implementation request has no approved spec.
---

# Write a spec

Produce a **draft** `spec.md` that says what and why, never how.

## Steps

1. **Read context**: `AGENTS.md` (Scope), `.specify/memory/constitution.md`
   (Principle I), `docs/development/spec-driven-workflow.md`, and
   `specs/README.md`. Check that no existing spec already covers the
   request; if one does, propose amending it instead.
2. **Pick the number**: list `specs/` directories that match `^[0-9]{3,}-`
   and take the highest number + 1, zero-padded to three digits. Ignore
   `_template`.
3. **Name it**: 2–4 word kebab-case short name, e.g. `042-cron-triggers`.
4. **Create** `specs/NNN-short-name/spec.md` by copying
   `specs/_template/spec.md`. In the front matter set
   `status: draft` and `feature: "NNN-short-name"`; fill the header
   (branch = directory name, today's date, the user's request as Input).
5. **Fill every section**:
   - *Problem*: who hurts and why now.
   - *User stories*: prioritized P1..Pn, each with an Independent Test and
     Given/When/Then scenarios.
   - *Functional requirements*: `FR-001…`, each one EARS sentence
     (`WHEN … THE SYSTEM SHALL …`, `IF … THEN …`, `WHILE …`, `WHERE …`),
     one behavior each, observable and testable.
   - *Quality, Safety & Compatibility*: process execution, secrets,
     cross-platform, contract impact.
   - *Edge cases*, *Success criteria*, *Non-goals* (include anything outside
     v0.1.0 scope that a reader might expect), *Assumptions*.
6. **Mark unknowns**: write `[NEEDS CLARIFICATION: question?]` inline for
   anything that would change behavior or architecture; at most 3. Never
   guess.
7. **Index it**: add a row to the table in `specs/README.md`
   (number, link, `draft`, affected public contract or `—`, one-line summary).
8. **Review**: run the `spec-reviewer` subagent on the new spec and address
   its findings.
9. **Report** the path, the open clarifications, and whether a public
   contract is affected (if so, an ADR is required before approval of the
   plan; see `write-adr`).

## Rules

- Do not write `plan.md` or `tasks.md` until the spec is `approved`, unless
  the user explicitly asks.
- Do not set `status: approved` yourself; approval is a maintainer decision.
- No package names, types, or libraries in `spec.md` unless externally
  observable.
- Commit as `docs(spec): NNN short name`.
