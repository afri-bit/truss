@AGENTS.md

## Claude Code

Project skills (`.claude/skills/`), invoked automatically or as `/name`:

- `write-spec`: start a new feature spec from the template and index it.
- `implement-task`: implement exactly one task from an approved spec.
- `write-adr`: record an architecture decision.
- `go-conventions`: the Go coding conventions with examples.
- `release`: changelog, version, and tag checklist.

Subagents (`.claude/agents/`):

- `spec-reviewer`: reviews a spec for testability, edge cases, and scope.
- `code-reviewer`: reviews a diff against the AGENTS.md rules.
- `docs-writer`: keeps user docs in sync with CLI and schema changes.
