package engine

import "time"

// StepEvents receives each started step.
// A nil StepEvents skips those calls.
// StepStarted is called once for every attempt of a Step.
// StepOutput receives command output as produced, without transcript text.
// StepFinished sets exitCode when the command ran, once for every attempt.
// startErr is set and exitCode is nil when the command did not start.
// StepStopped is called only for a Step TRUSS stopped because failedID failed,
// after its last output and before its StepFinished. A Step that was waiting
// to retry when that happened gets a StepStopped and no StepFinished.
// StepRetrying follows the StepFinished of a failed attempt that will be run
// again: attempt is the number of the next attempt out of attempts, and delay
// is the wait before it. The next attempt begins with another StepStarted.
// StepContinued follows the StepFinished of a failed Step whose
// continue-on-error let the run go on.
// StepSkipped reports a Step that will not run and says why, as a phrase such
// as "step a did not succeed". It is not called for a Step that was never
// started only because an earlier failure ended the run.
type StepEvents interface {
	StepStarted(id string)
	StepOutput(id string, chunk []byte)
	StepStopped(id, failedID string)
	StepFinished(id string, exitCode *int, startErr error)
	StepRetrying(id string, attempt, attempts int, delay time.Duration)
	StepContinued(id string)
	StepSkipped(id, reason string)
}
