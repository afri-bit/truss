# Contract: Run log

**Status**: Replaces the unreleased transcript in `specs/001-host-pipeline-engine/contracts/cli.md`  
**Compatibility**: No version has been tagged. Schema version `"1"` and sequential fail-fast execution are unchanged. A successful step now shows its start, finish, duration, and Exit Code. Every line TRUSS writes carries a timestamp. The failure line is part of this log.

## Line

```text
YYYY-MM-DD HH:MM:SS <occurrence>
```

The timestamp is the machine's local date and time, to one second. It is never earlier than the timestamp on the previous line. Duration is separate and uses seconds with three digits, such as `0.040s` or `65.200s`.

## Setup

These five lines appear first, before any step line:

```text
<ts> setup environment ready
<ts> setup os <linux|macOS|windows|other reported name>
<ts> setup arch <architecture>
<ts> setup shell <host shell path>
<ts> setup directory <absolute working directory>
```

No other setup line is written. Environment variable names, environment variable values, and secrets are absent.

## Step

For each started step, in definition order:

```text
<ts> step <id> started
<ts> step <id> | <output line>
<ts> step <id> finished duration <seconds>s exit <code>
```

- Output lines follow `started` and precede `finished`. Each command line, including a blank line, is one occurrence. The text after `| ` is the command's line without a trailing newline.
- A step that writes nothing has no `|` line.
- Standard output and standard error of that step appear in the order they are presented.
- A command that does not start uses this finish line and no `exit` field:

```text
<ts> step <id> finished duration <seconds>s error <diagnostic>
```

## Summary

After the last started step:

```text
<ts> summary succeeded steps <n> duration <seconds>s
```

or, when the run failed:

```text
<ts> summary failed steps <n> duration <seconds>s
<ts> error <pipeline name> failed at <id> (exit <code>)
```

When the command did not start:

```text
<ts> error <pipeline name> failed at <id>: <diagnostic>
```

`<n>` is the number of steps that started. Later steps are omitted and are not marked succeeded.

## Saved copy

The bytes above are the whole run log. `.truss/run.log` in the invoker's working directory contains those bytes and nothing else. The next finished run replaces the file.
