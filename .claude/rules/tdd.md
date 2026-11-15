# Tests ship with behavior

Constitution principle III (non-negotiable). Applies to every behavioral change.

- New or changed behavior comes with tests in the same change.
- Writing the test first is the recommended way to work: write it, watch it fail for the right
  reason, implement, refactor. A regression fix should start from a test that reproduces the defect.
- Tests live in `*_test.go` beside the code. Assert observable behavior, not private structure.
  Fixtures go in `testdata/`.
- Process execution, CLI contracts, the daemon API and other public interfaces need integration or contract tests,
  not only unit tests of internals. The built-binary end-to-end script is `.github/scripts/e2e-test.sh`.
- Never weaken, skip or delete a test to get green. If a test is wrong, say why in the PR.
- Mechanical changes with no observable effect (formatting, typos, equivalent refactors) need no
  new test.
- Run `go test -race` for the affected packages before calling a task done.

```go
func TestRun_StopsAfterFailedStep(t *testing.T) {
    result := runPipeline(t, twoStepsSecondFails)
    if result.Status != Failed { t.Fatalf("status = %s", result.Status) }
    if len(result.Steps) != 1 { t.Fatalf("later step ran: %d", len(result.Steps)) }
}
```
