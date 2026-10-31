---
paths:
  - "specs/**"
  - ".specify/**"
  - "docs/MILESTONE.md"
---

# Specifications

Principle I: required behavior lives in `specs/<feature>/spec.md` before implementation.

- `spec.md` holds what and why, with independently testable user stories. `plan.md` holds
  technical decisions. `tasks.md` holds the work. Do not put implementation detail in a spec
  unless it is observable or an explicit constraint.
- Generate artifacts only with the `/speckit-*` skills. Never hand-write `plan.md`, `tasks.md`,
  `research.md`, `data-model.md`, `contracts/` or `quickstart.md`, and never rebuild them after
  code exists. Tick tasks in `tasks.md` as they finish.
- Read `docs/MILESTONE.md` first and stay inside the current release. A feature outside it needs
  the maintainer's decision, not an agent's.
- `.specify/` is Spec Kit infrastructure. Do not edit generated scripts or templates unless the
  task concerns that tooling; a template change is recorded as a constitution amendment.
- Resolve material ambiguity with `/speckit-clarify` or an open question. Do not choose
  arbitrarily and do not invent requirements.
- If code and spec disagree, name the discrepancy before changing either one.
- Branch for a spec is `feature/<NNN-name>` and matches the `specs/` folder. Spec Kit 1.2.x does not
  infer the spec folder from the branch name. `/speckit-specify` writes the active folder to
  `.specify/feature.json` (gitignored, per checkout). Create the branch with
  `GIT_BRANCH_NAME=feature/<NNN-name>`, and on a fresh checkout or another machine set
  `SPECIFY_FEATURE_DIRECTORY=specs/<NNN-name>` before `/speckit-plan`, `/speckit-tasks` or
  `/speckit-implement`.
