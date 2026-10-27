package pipeline

import (
	"fmt"
	"strings"
)

// Validate checks a parsed pipeline against schema v1. It does not execute steps.
// Structural errors are reported before dependency errors.
func Validate(p Pipeline) error {
	if p.nonStringVersion || p.Version != "1" {
		if strings.TrimSpace(p.Version) == "" {
			return fmt.Errorf("missing or unsupported schema version")
		}
		return fmt.Errorf("unsupported schema version %q", p.Version)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("missing pipeline name")
	}
	if p.stepsNotList {
		return fmt.Errorf("steps must be a list")
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("pipeline must contain one or more steps")
	}
	index := make(map[string]int, len(p.Steps))
	for i, step := range p.Steps {
		id := strings.TrimSpace(step.ID)
		if id == "" {
			return fmt.Errorf("missing step id")
		}
		if _, ok := index[id]; ok {
			return fmt.Errorf("duplicate step id %q", id)
		}
		index[id] = i
		if step.runNotString || strings.TrimSpace(step.Run) == "" {
			return fmt.Errorf("missing or empty run for step %q", id)
		}
	}
	return validateNeeds(p.Steps, index)
}

// validateNeeds checks the declared dependencies. index maps each step id to
// its position in steps. The first fault in definition order is reported, and
// within one list the first faulty entry is reported. Cycles are reported only
// once every list is otherwise valid.
func validateNeeds(steps []Step, index map[string]int) error {
	for _, step := range steps {
		id := strings.TrimSpace(step.ID)
		if step.needsInvalid {
			return fmt.Errorf("step %q needs must be a list of step ids", id)
		}
		seen := make(map[string]struct{}, len(step.Needs))
		for _, need := range step.Needs {
			if _, ok := index[need]; !ok {
				return fmt.Errorf("step %q needs unknown step %q", id, need)
			}
			if need == id {
				return fmt.Errorf("step %q depends on itself", id)
			}
			if _, ok := seen[need]; ok {
				return fmt.Errorf("step %q repeats step %q", id, need)
			}
			seen[need] = struct{}{}
		}
	}
	if cycle := findCycle(steps, index); len(cycle) > 0 {
		return fmt.Errorf("dependency cycle: %s", strings.Join(cycle, ", "))
	}
	return nil
}

// findCycle returns the ids of one dependency cycle, or nil when there is
// none. It searches steps in definition order and each needs list in entry
// order. The result starts at the member written earliest and then follows
// needs, so each id needs the next and the last id needs the first.
func findCycle(steps []Step, index map[string]int) []string {
	const (
		unvisited = iota
		onPath
		done
	)
	state := make([]int, len(steps))
	var path, cycle []int

	var visit func(i int) bool
	visit = func(i int) bool {
		state[i] = onPath
		path = append(path, i)
		for _, need := range steps[i].Needs {
			j := index[need]
			switch state[j] {
			case onPath:
				start := 0
				for path[start] != j {
					start++
				}
				cycle = append(cycle, path[start:]...)
				return true
			case unvisited:
				if visit(j) {
					return true
				}
			}
		}
		path = path[:len(path)-1]
		state[i] = done
		return false
	}

	for i := range steps {
		if state[i] == unvisited && visit(i) {
			break
		}
	}
	if len(cycle) == 0 {
		return nil
	}

	first := 0
	for k, i := range cycle {
		if i < cycle[first] {
			first = k
		}
	}
	ids := make([]string, 0, len(cycle))
	for k := range cycle {
		ids = append(ids, strings.TrimSpace(steps[cycle[(first+k)%len(cycle)]].ID))
	}
	return ids
}
