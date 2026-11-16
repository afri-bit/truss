# Contract: Run log

**Status**: Extends `specs/002-local-run-log/contracts/run-log.md`  
**Compatibility**: Line format, setup, timestamps, durations, summary, failure line, and `.truss/run.log` are unchanged. *Changed by [spec 016](../../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.* The only change is step order: started Steps appear in execution order. When no Step has a non-empty `needs`, that order is still definition order. The log does not gain a `needs` field. No version has been tagged.

## Step order

The step section of `specs/002-local-run-log/contracts/run-log.md` applies, with this replacement:

For each started step, in execution order:

```text
<ts> step <id> started
<ts> step <id> | <output line>
<ts> step <id> finished duration <seconds>s exit <code>
```

Execution order is [execution.md](./execution.md). A Step that does not start is absent and is not marked succeeded. No line lists `needs` or any other Dependency.

`<n>` in the summary remains the number of Steps that started.
