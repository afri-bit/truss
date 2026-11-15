---
paths:
  - "**/*.go"
---

# Go style

Follow [Google's Go Style Guide](https://google.github.io/styleguide/go/). Principle II of
`.specify/memory/constitution.md` is the authority; this file is the short form. Run `gofmt` and
`go vet` on every change.

## Naming

- MixedCaps, never underscores. Initialisms stay capitals: `URL`, `ID`, `HTTP`.
- Package names are short, lowercase and singular. No `util` or `common` dumping grounds.
- No stutter: `exec.Runner`, not `exec.ExecRunner`.
- Receivers are one or two letters (`r *Runner`), never `this` or `self`.

## Types and errors

- Name the types at an API boundary. Avoid `any` and `interface{}` unless the boundary is
  genuinely untyped (for example YAML decoding).
- `context.Context` is the first parameter of anything that blocks, runs a process, does I/O or
  must be cancellable. Never store a context in a struct.
- Return errors; wrap them with `fmt.Errorf("what failed: %w", err)`. Compare with `errors.Is`
  and `errors.As`. Never `panic` on user input or runtime failure.
- No global mutable state without a demonstrated need.

## GoDoc

Every exported name has a complete-sentence comment that starts with the name. The package has a
`// Package x ...` comment. Unexported code is commented only where the behavior is not obvious.
Comments are English, wrapped near 80 columns, no HTML.

```go
// Step is a single executable unit in a pipeline.
type Step struct {
    ID  string
    Run string
}
```

## Dependencies

Prefer the standard library. A third-party module needs a concrete problem it solves and a
mention in the plan (constitution principle IV). `gopkg.in/yaml.v3` is the only one today.
