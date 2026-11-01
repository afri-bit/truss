# Quickstart: Step Concurrency

Validation guide. Contracts: [contracts/](./contracts/). Data: [data-model.md](./data-model.md).

## Prerequisites

Go 1.27.3, a clone of the repository, and a shell. Build once: `go build -o bin/truss ./cmd/truss`.

## 1. Steps run together (Story 1)

`truss.yaml`:

```yaml
version: "1"
pipeline:
  name: together
  concurrency: 2
steps:
  - id: a
    run: <command that creates a.ready, waits until b.ready exists, then exits 0>
  - id: b
    run: <command that creates b.ready, waits until a.ready exists, then exits 0>
```

Run `bin/truss run`. Expect exit 0 and both Steps started before either finished. With `concurrency` omitted the same file does not complete (each Step waits for the other). The automated tests use a helper process for the two commands so they work under `sh` and `cmd.exe`.

## 2. One readable log (Story 2)

Two overlapping Steps that print several lines. Expect every output line labelled `step <id> |`, no two Steps' text in one line, non-decreasing timestamps, and `.truss/run.log` equal to what was printed.

## 3. Failure stops the rest (Story 3)

Limit 2: `a` exits 1 soon after start; `b` runs long and would write a marker file if it finished; `c` needs nothing and would write another marker. Expect exit 1, no marker file, `b` with a `stopped because step a failed` line, `c` absent, and no process from `b` left running (check the pid file the helper wrote).

Interrupt: start a long run and press Ctrl-C. Expect exit 1, no Step process left, `.truss/run.log` unchanged.

## 4. Invalid input is rejected (Story 4)

For `concurrency: 0`, `-1`, `1.5`, `"two"`, `true`, `[]`, empty, and for a misspelled `concurency:`, with a `.truss/run.log` already present: expect `error: ...` naming the field, exit 1, no command run, and the log byte-for-byte unchanged.

## 5. Unchanged without a limit (Story 5)

Run the spec 003 definitions with no `concurrency` and with `concurrency: 1`. Expect the spec 003 orders, fail-fast omission, and log shape. A Step that starts a background process and exits: the background process is ended when the Step finishes.

## Automated

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -count=20 -run 'Concurren|Stop|Tree' ./internal/engine ./internal/executor ./cmd/truss
golangci-lint run ./...
go build -o bin/truss ./cmd/truss && bash .github/scripts/e2e-test.sh bin/truss
```
