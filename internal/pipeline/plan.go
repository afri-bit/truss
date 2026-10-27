package pipeline

import "fmt"

// Plan returns a sequential execution plan. A step is placed only after every
// step it needs. When several steps could be placed next, the one written
// earliest in the definition goes first, so a definition without needs keeps
// definition order. Each planned step keeps only its declared needs; the
// tie-break is order, not a dependency.
func Plan(p Pipeline) (ExecutionPlan, error) {
	steps := make([]Step, 0, len(p.Steps))
	placed := make(map[string]bool, len(p.Steps))
	used := make([]bool, len(p.Steps))
	for len(steps) < len(p.Steps) {
		next := -1
		for i, step := range p.Steps {
			if !used[i] && ready(step, placed) {
				next = i
				break
			}
		}
		if next < 0 {
			return ExecutionPlan{}, fmt.Errorf("cannot plan steps: no step is ready to run")
		}
		used[next] = true
		placed[p.Steps[next].ID] = true
		steps = append(steps, p.Steps[next])
	}
	return ExecutionPlan{Name: p.Name, Steps: steps}, nil
}

// ready reports whether every step that s needs has already been placed.
func ready(s Step, placed map[string]bool) bool {
	for _, id := range s.Needs {
		if !placed[id] {
			return false
		}
	}
	return true
}
