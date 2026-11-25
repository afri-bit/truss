# Feature Specification: A neat command line

**Spec**: `025-cli-tidy` | **Created**: 2026-11-25 | **Status**: Implemented

Extends [`002-local-run-log`](../002-local-run-log/spec.md) (usage, FR-014) and
[`022-cli-enrichment`](../022-cli-enrichment/spec.md). User docs: [`docs/cli.md`](../../docs/cli.md).
Decision: [ADR 0022](../../docs/decisions/0022-errors-say-what-to-do-next.md). Risk class: tier 2
(CLI usage and output). Scope set by the maintainer on 2026-11-25: the commands keep their names
and are tidied, not renamed.

## Why

A typo or a wrong argument printed the whole usage, 50 lines, and the user had to find their
mistake in it. Advice was buried in parentheses at the end of an error. There was no completion.

## User stories

1. **A typo is caught** (P1). I type `truss regster` and TRUSS says it does not know the command and
   suggests `register`.
2. **Wrong arguments show the right usage** (P1). I type `truss daemon restart` and see the usage of
   `truss daemon`, with a suggestion, not the whole list.
3. **Errors say what to do** (P1). An error that has advice shows it on a `hint:` line.
4. **Completion** (P2). I press Tab after `truss` in bash, zsh, fish or PowerShell and get commands,
   subcommands and my flow names.
5. **Exit codes are written down** (P2). `truss help exit-codes` lists them.

## Requirements

- **FR-001** An unknown first word prints `error: unknown command "<word>"`, then
  `hint: did you mean "<command>"?` when a command is at most one edit away (two for words longer
  than five letters), then `usage: truss <command> [arguments]` and a pointer to `truss help`; exit 1.
  `truss help <unknown>` behaves the same.
- **FR-002** Wrong arguments to a known command print `error: ...`, then `usage:` with the synopsis
  of that command's help topic (a command without one uses the topic it shares, for example
  `pause` uses `register`), then `Run "truss help <topic>" for details.`; exit 1. An unknown
  subcommand close to a known one is named with a suggestion. A command with no topic prints the
  whole usage, as before.
- **FR-003** An error whose message ends with advice in parentheses that contains ` with: ` prints
  the message on the `error:` line and the advice on a `hint:` line.
- **FR-004** `truss completion bash|zsh|fish|powershell` prints a completion script for commands,
  subcommands (and, after `runs`, `show` or a flow) and the names of the `.yaml` and `.yml`
  files of `.truss/flows/` in the current folder; any other shell is a usage error. `truss version`
  and `truss help` with extra words print the whole usage, as before. Errors of `truss run` get
  the hint line too.
- **FR-005** `truss runs -n <n>` is `--limit <n>`; giving both is an error.
- **FR-006** `truss help exit-codes` lists the exit codes: 0 success, 1 anything else.
- **FR-007** Command names, flags, standard output, JSON and exit codes are unchanged, except the
  help text, which lists `completion` and `-n`. TRUSS prints no colour.
- **FR-008** Completion treats file names as data: nothing in a name is run or expanded, names are
  inserted quoted, and names starting with `-` are not offered.

## Compatibility

Only standard error changes, for usage errors and for errors with advice. Exit codes stay. Spec 002
FR-014 ("print usage, exit non-zero") still holds: the usage printed is the command's own.

## Tested where

The bash script is parsed and exercised in the Go tests on Unix. zsh, fish and PowerShell
scripts are parsed when the shell is installed (CI's Windows runner has pwsh); they are not
exercised by a test.

## Not done

`-q/--quiet` and `--no-color` were proposed; TRUSS prints no colour, and no command has output a
quiet mode would remove without hiding the result, so neither was added.
