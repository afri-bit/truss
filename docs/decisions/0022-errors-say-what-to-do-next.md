# 0022. Errors say what to do next

- Status: Proposed
- Date: 2026-11-25

## Context

A typo or wrong arguments printed the whole usage (spec 002 FR-014). Advice was in parentheses at
the end of an error. The maintainer asked for a neater CLI without renaming commands (milestone
v0.3.0, 2026-11-25). CLI output is a public contract (constitution V).

## Decision

1. An unknown command is named, with a suggestion by edit distance when one is close.
2. Wrong arguments print the command's own usage synopsis, not the whole list.
3. Advice moves to a `hint:` line. Errors keep their `error:` prefix and exit code 1.
4. `truss completion <shell>` generates scripts from one table of commands; a test keeps that
   table in step with the dispatcher.
5. Standard error text is for people and may change in minor versions; scripts use `--json` and
   exit codes.

## Consequences

- Shorter, more useful errors; no command, flag or exit code changes.
- Tests that compared standard error with the full usage now check for an error and a usage line.
