# Contract: Run log

**Status**: Extends `specs/003-step-dependencies/contracts/run-log.md` and `specs/002-local-run-log/contracts/run-log.md`  
**Compatibility**: Setup, timestamp format, durations, summary, failure line, and `.truss/run.log` are unchanged. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.* One new line type (`stopped`) appears only when a Step is stopped. Output lines from different Steps may alternate when Steps overlap. The log does not show the limit or any Dependency. No version has been tagged.

## Lines

```text
<ts> step <id> started
<ts> step <id> | <output line>
<ts> step <id> stopped because step <other> failed
<ts> step <id> finished duration <seconds>s exit <code>
<ts> step <id> finished duration <seconds>s error <diagnostic>
```

## Rules

- Every output line is one complete line with exactly one `step <id> |` label. A partial line is held until its newline or the Step's end, then written once.
- Lines from different Steps may alternate. Each Step's own lines appear in the order it wrote them.
- A timestamp is never earlier than the previous line's.
- Started Steps appear in the order they started.
- `stopped` appears only for a Step ended because another Step failed, after that Step's output and before its finish line, and names the Step whose failure TRUSS observed first.
- A Step with no host-reported Exit Code (start failure, signal death) ends with `error <diagnostic>` and no `exit`. `exit -1` is never written.
- The summary's `<n>` is the number of Steps that started.
- The text shown while running is the text saved to `.truss/run.log`. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*
