# Contract: Run log, headings

**Status**: Extends `specs/002-local-run-log/contracts/run-log.md`, `specs/003-step-dependencies/contracts/run-log.md` and `specs/004-step-concurrency/contracts/run-log.md`  
**Compatibility**: Additive to the line grammar, but it changes the released v0.1.0 log: the first line is now a heading instead of `setup environment ready`, and a heading also comes before each step attempt and before the summary. Every other line is unchanged. Needs the maintainer's explicit acceptance (see the spec and ADR 0018).

## Heading

```text
<ts> ==== <title> ====
```

Three headings exist, in this place:

```text
<ts> ==== Setup: the environment the steps run in ====
<ts> setup environment ready
<ts> setup os ...
...
<ts> ==== Step <id> ====
<ts> step <id> started
<ts> step <id> | <output line>
<ts> step <id> finished duration <seconds>s exit <code>
...
<ts> ==== Summary ====
<ts> summary succeeded steps <n> duration <seconds>s
```

## Rules

- The setup heading is the first line. The setup section is the heading and the five lines of
  spec 002; no other setup line is written.
- A step heading comes immediately before that step's `started` line and carries the same
  timestamp. A step that is tried again gets a heading for every attempt. A step that does not
  start (skipped) has none.
- The summary heading comes immediately before the `summary` line. The `error` line of a failed run
  follows the summary line.
- A heading starts with `==== `. No other line TRUSS writes starts with it, and command output
  always starts with `step <id> | `, so it cannot be a heading.
- `<id>` is written as the pipeline file gives it. The schema does not restrict the characters of
  a step id yet (a follow-up), so an id that holds a line break, ` | ` or `====` can forge lines. The
  contract holds for ids made of letters, digits and `_ . - /`; a log of a file with other ids may
  not be trusted to tell the parts apart.
- Every line still has a timestamp, and timestamps still never go backwards.
- Lines of different steps may alternate (spec 004). A heading marks where a step starts, not an
  exclusive section.
