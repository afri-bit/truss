# Feature Specification: A readable run log

**Spec**: `020-readable-run-log` | **Created**: 2026-11-16 | **Status**: Implemented

Extends [`002-local-run-log`](../002-local-run-log/spec.md) (the run log),
[`004-step-concurrency`](../004-step-concurrency/spec.md) (steps that overlap) and
[`014-desktop-redesign`](../014-desktop-redesign/spec.md) (the run page). Contract:
[`contracts/run-log.md`](contracts/run-log.md). User docs:
[`docs/cli.md`](../../docs/cli.md#reading-the-run-log), [`docs/desktop.md`](../../docs/desktop.md#the-run-page).
Decision: [ADR 0018](../../docs/decisions/0018-the-run-log-gets-headings-structure-for-machines-is-separate.md).
Risk class: tier 2 (the run-log format and CLI output). Scope decided by the maintainer on
2026-11-16: the CLI solves it in the console text, Desktop with a UI.

## Problem

The log is one flat list. A line TRUSS writes about the machine (`setup os macOS`), a line about a
step (`step stress started`) and a line a command printed (`step stress | stressing done`) look
alike, nothing says where one part ends, and nothing says what the setup lines are. Jenkins and
GitHub Actions group the log by step and say what each part is.

## User stories

1. **Tell TRUSS from my commands** (P1). In the terminal I can see where the setup, each step and
   the summary begin, and that setup lines describe the environment my steps run in.
2. **One card per step** (P1). In Desktop I see the log as cards I can open and close: the setup,
   one card per step, the summary. The step that failed is open; the ones that went well are closed.
3. **Old logs still read** (P2). A log written before this change (a saved `run.log`, a run in the
   daemon's history) shows as cards in Desktop too.

## Requirements

- **FR-001** The run log opens each section with a **heading** line: `==== Setup: the environment
  the steps run in ====` before the setup lines, `==== Step <id> ====` before a step's start line
  (once per attempt), and `==== Summary ====` before the summary line. A heading carries the usual
  timestamp, so every line of the log still does.
- **FR-002** A heading starts with `==== ` and never with `step`. Output a command prints is
  always written as `step <id> | <text>`, so a command that prints `==== Summary ====` produces a
  step-output line, not a heading. This holds for command output, not for a step id (see Known limits).
- **FR-003** Nothing else changes: the setup lines, every step line, the summary and the error line
  keep their text and order. A reader or script that looks for those lines still finds them. The
  saved `run.log`, the text `truss run` prints, `truss logs` and the daemon's log are one text
  (spec 002 FR-007).
- **FR-004** Desktop's run page shows the log as cards: **Setup**, one per step, **Summary**. A step's
  card is built from every line labelled with that step, so overlapping steps do not mix. The
  header shows a state glyph, the step id and its duration (`running` while it runs, the reason
  when it was skipped); the setup's header shows OS, architecture and shell. A step cut off by a
  cancel shows as cancelled. A step id that holds a space is matched against the run's step ids.
- **FR-005** A card is open when its step is running, failed, stopped or failed-and-continued, and
  for the summary. It is closed for a step that succeeded or was skipped, and for the setup. Clicking
  a header opens or closes the card, and the choice stays while the log keeps arriving. Two buttons
  open or close every card.
- **FR-006** Picking a step shows only its card, open. Finding text shows only the cards that have
  a match, open, with the match marked. Lines inside a card are numbered from 1.
- **FR-007** Desktop builds cards from the step, setup and summary lines (spec 002 to 004
  grammar), and skips headings. A log without headings, written by an earlier build, gives the
  same cards.

## Known limits

- The headings are for people. They are not a machine format: a tool that wants structured data
  should not parse them (see ADR 0018 for what comes next).
- With overlapping steps a step's heading is followed by lines of other steps until its own output
  arrives; every line stays labelled, so none is misread.
- A step id may hold any character the schema allows, including spaces, a line break or ` | `.
  Such an id can forge heading, summary or step lines in the log (the exit code stays right).
  Desktop copes with a space or ` | ` in an id by matching the run's step ids, but not with a
  forged line. Restricting step ids is a schema change and a follow-up with its own ADR.
- Desktop counts the lines it shows; the headings are not counted, so the number can differ from
  the line count of `run.log`.
- The terminal output has no colour: a colour would have to be left out of the saved copy, which
  must equal the shown text.

## Compatibility

Additive, so no contract of a release is broken. The line shape `<timestamp> <text>`, the setup
lines, the step lines, the summary, the error line, the saved file's place, the API and the exit
codes stay as they were. New: the heading lines. A consumer that treats every line it does not know
as noise keeps working; one that counted lines or expected the first line to be
`setup environment ready` must skip the heading. The repository's own checks that did so were
updated.

## Verification

`internal/runlog` tests (headings, spoofing, concurrency), `cmd/truss` log-shape tests, the
built-binary end-to-end script, and the Desktop browser tests (`tests/ui/run.test.js`: cards,
open and closed states, search, a closed card staying closed, a log without headings).
