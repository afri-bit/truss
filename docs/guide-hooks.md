# Start a flow when something happens

Start a flow when files change, or when another flow ends, instead of on a clock.

Hooks act while the [daemon](daemon.md) runs, for registered flows that are not paused:
`truss register --all && truss daemon start`.

## When files change: `on.file`

Sort photos as soon as they land in a folder:

```yaml
# .truss/flows/sort.yaml
version: "1"
flow:
  name: sort
on:
  file:
    paths: [inbox/]                 # a folder, a file, or a pattern like "src/**/*.go"
    ignore: ["**/*.tmp"]            # optional
    debounce: 5s                    # optional: wait until changes stop (default 2s)
steps:
  - id: sort
    run: ./sort-photos.sh < "$TRUSS_CHANGED_FILES"
```

- A burst of changes is **one** run, once nothing changed for `debounce`.
- `$TRUSS_CHANGED_FILES` names a file that lists what changed, one path per line, relative to the
  project root.
- Paths are relative to the project root, or absolute (`/srv/inbox`, `C:/inbox`). Use `/` on
  every system. `*` and `?` match within a name, `**` across folders.
- Changes made while the daemon was stopped, or while the flow was paused, start nothing.

Keep the output of the flow out of the folders it watches, or `ignore` it, or it starts itself.

## When another flow ends: `on.run`

Deploy after every successful build; alert when it fails:

```yaml
# .truss/flows/deploy.yaml
on:
  run:
    flow: build                     # status: succeeded is the default
```

```yaml
# .truss/flows/alert.yaml
on:
  run:
    flow: build
    status: failed                  # succeeded, failed, any, or a list
```

`on.run` may be a list to follow several flows.

## Safety limits

- A flow has at most one run started by a hook at a time. More changes or ended runs meanwhile
  start one run after it, not one each.
- Flows that start each other in a loop stop after 10 runs in a row.
- A scan stops at 50,000 files and folders per flow; narrow `paths` or `ignore` big folders such
  as `node_modules`.
- A whole disk, `~` and `..` cannot be watched.

The daemon log (`truss daemon logs`) says when a limit was hit. `truss flows show sort` lists a
flow's hooks. The [flow file reference](pipeline-reference.md#onfile-start-when-files-change) has
every rule.
