package pipeline

// Plan returns a sequential execution plan in definition order.
func Plan(p Pipeline) (ExecutionPlan, error) {
	steps := make([]Step, len(p.Steps))
	copy(steps, p.Steps)
	return ExecutionPlan{Name: p.Name, Steps: steps}, nil
}
