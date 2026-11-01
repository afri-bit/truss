# Test-first development

Constitution principle III (non-negotiable). Applies to every behavioral change.

1. Write or update a test for the specified observable outcome.
2. Run it and confirm it fails because the behavior is missing or wrong. Keep that output.
3. Implement only enough to make it pass.
4. Refactor while the tests stay green.

- No production behavior before a failing test exists for it. A regression fix starts from a test
  that reproduces the defect.
- Tests live in `*_test.go` beside the code. Assert observable behavior, not private structure.
  Fixtures go in `testdata/`.
- Process execution, CLI contracts and other public interfaces need integration or contract tests,
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
