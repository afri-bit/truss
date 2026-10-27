package engine

// StepEvents receives each started step.
// A nil StepEvents skips those calls.
// StepOutput receives command output as produced, without transcript text.
// StepFinished sets exitCode when the command ran.
// startErr is set and exitCode is nil when the command did not start.
type StepEvents interface {
	StepStarted(id string)
	StepOutput(id string, chunk []byte)
	StepFinished(id string, exitCode *int, startErr error)
}
