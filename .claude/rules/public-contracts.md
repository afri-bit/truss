---
paths:
  - "specs/**/contracts/**"
  - "internal/pipeline/**"
  - "internal/runlog/**"
  - "cmd/truss/**"
  - "README.md"
  - "CHANGELOG.md"
---

# Public contracts

The pipeline file (schema `version`), CLI usage and exit codes, and the run-log format are
versioned interfaces (constitution principle V).

- Identify the compatibility impact in the plan **before** implementing a change to any of them.
  An incompatible change to an established contract needs an explicit decision (`/adr`) and a
  schema version or release-note plan.
- Contracts are described in `specs/<feature>/contracts/` and in README sections; keep both and
  the tests in agreement. When code and a contract disagree, report the discrepancy first; do not
  silently change either side.
- Record user-visible changes under `[Unreleased]` in `CHANGELOG.md`, in the Keep a Changelog
  style already used there. `go.mod`'s `// version:` line and the release check own version numbers.
- Do not leak Go types, build tags or module paths into a contract.
