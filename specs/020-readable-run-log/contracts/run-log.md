# Contract: Run log, headings

**Status**: Extends `specs/002-local-run-log/contracts/run-log.md`, `specs/003-step-dependencies/contracts/run-log.md` and `specs/004-step-concurrency/contracts/run-log.md`  
**Compatibility**: Additive. Three heading lines are new; every other line is unchanged. No release carried a log without headings.

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
- `<id>` is a step id as validated by the pipeline schema; it holds no line break.
- Every line still has a timestamp, and timestamps still never go backwards.
- Lines of different steps may alternate (spec 004). A heading marks where a step starts, not an
  exclusive section.
