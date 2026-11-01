package pipeline

import "testing"

// Compile-level check that the run model carries the first failed step and
// the step that stopped another.
func TestRunModel_RecordsFailedAndStoppingStep(t *testing.T) {
	run := PipelineRun{Name: "n", Status: StatusFailed, FailedStep: "build"}
	step := StepRun{ID: "test", Status: StatusFailed, StoppedBy: "build"}
	if run.FailedStep != "build" || step.StoppedBy != "build" {
		t.Fatalf("run = %+v, step = %+v", run, step)
	}
}
