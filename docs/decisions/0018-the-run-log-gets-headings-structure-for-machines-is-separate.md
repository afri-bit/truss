# 0018. The run log gets headings; structure for machines is a separate decision

- Status: Proposed
- Date: 2026-11-16

## Context

The run log is one flat list of lines. Nothing marks where the setup, a step or the summary begins,
and a line TRUSS writes looks like a line a command printed. The maintainer asked for the terminal
to fix this in the text and for Desktop to fix it with a UI. Spec 002 requires the saved log, the
printed log and the daemon's log to be one text, so a separate terminal-only rendering would break
that rule.

## Decision

Add three timestamped heading lines to the text itself (`==== Setup: ... ====`, `==== Step <id>
====`, `==== Summary ====`) and change nothing else in it. They are plain ASCII, so any terminal
and any log tool shows them. A heading never starts with `step`, so command output, always
`step <id> | ...`, cannot pass for one. Desktop builds its cards from the existing step, setup and
summary lines and skips headings, so logs without headings group the same way. Desktop does not
need a new endpoint.

The headings are for readers. A machine-readable format (structured events for Grafana, Loki or
OpenTelemetry) is not decided here; it is the next discussion, and the text log stays the human
view of it.

## Consequences

- It changes the log of the released v0.1.0 (a new first line and two more), so it needs the maintainer's explicit acceptance.
- Additive in grammar: scripts that search for known lines keep working; a script that counts lines or
  expects `setup environment ready` first must skip the heading. The repository's own tests
  were updated.
- Docs and the README samples show the headings.
- No colour in the terminal: the shown text must equal the saved text.
- If a structured format arrives, the headings may become redundant there. They stay in the text
  log either way.
