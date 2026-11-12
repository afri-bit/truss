package engine

// StepEvents receives each started step.
// A nil StepEvents skips those calls.
// StepOutput receives command output as produced, without transcript text.
// StepFinished sets exitCode when the command ran.
// startErr is set and exitCode is nil when the command did not start.
// StepStopped is called only for a Step TRUSS stopped because failedID failed,
// after its last output and before its StepFinished.
type StepEvents interface {
	StepStarted(id string)
	StepOutput(id string, chunk []byte)
	StepStopped(id, failedID string)
	StepFinished(id string, exitCode *int, startErr error)
}
