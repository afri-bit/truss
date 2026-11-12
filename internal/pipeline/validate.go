package pipeline

import (
	"errors"
	"fmt"
	"strconv"
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
	if p.unknownField != "" {
		return fmt.Errorf("unknown field %q %s", p.unknownField, p.unknownIn)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("missing pipeline name")
	}
	if p.concurrencyBad || p.Concurrency < 0 {
		text := p.concurrencyText
		if !p.concurrencyBad {
			text = strconv.Quote(strconv.Itoa(p.Concurrency))
		}
		return fmt.Errorf("pipeline concurrency must be a whole number of at least 1, got %s", text)
	}
	if len(p.issues) > 0 {
		return errors.New(p.issues[0])
	}
	if p.hasSteps && p.hasJobs {
		return fmt.Errorf("a pipeline has either steps or jobs, not both")
	}
	if p.hasJobs || len(p.Jobs) > 0 {
		return validateJobs(p.Jobs)
	}
	if p.stepsNotList {
		return fmt.Errorf("steps must be a list")
	}
	if len(p.Steps) == 0 {
		return fmt.Errorf("pipeline must contain one or more steps")
	}
	index, err := validateSteps(p.Steps, "")
	if err != nil {
		return err
	}
	return validateNeeds("step", p.Steps, index)
}

// validateSteps checks ids and run strings of one steps list and returns the
// position of each id. job is the owning job id, or empty.
func validateSteps(steps []Step, job string) (map[string]int, error) {
	index := make(map[string]int, len(steps))
	for i, step := range steps {
		id := strings.TrimSpace(step.ID)
		if id == "" {
			if job != "" {
				return nil, fmt.Errorf("missing step id in job %q", job)
			}
			return nil, fmt.Errorf("missing step id")
		}
		if _, ok := index[id]; ok {
			return nil, fmt.Errorf("duplicate step id %q", joinID(job, id))
		}
		index[id] = i
		if step.runNotString || strings.TrimSpace(step.Run) == "" {
			return nil, fmt.Errorf("missing or empty run for step %q", joinID(job, id))
		}
	}
	return index, nil
}

func validateJobs(jobs []Job) error {
	if len(jobs) == 0 {
		return fmt.Errorf("pipeline must contain one or more jobs")
	}
	index := make(map[string]int, len(jobs))
	nodes := make([]Step, len(jobs))
	for i, job := range jobs {
		if len(job.Steps) == 0 {
			return fmt.Errorf("job %q must contain one or more steps", job.ID)
		}
		if _, err := validateSteps(job.Steps, job.ID); err != nil {
			return err
		}
		index[job.ID] = i
		nodes[i] = Step{ID: job.ID, Needs: job.Needs}
	}
	return validateNeeds("job", nodes, index)
}

// validateNeeds checks the declared dependencies of steps or jobs; kind is
// that word. index maps each id to its position in steps. The first fault in
// definition order is reported, and within one list the first faulty entry is
// reported. Cycles are reported only once every list is otherwise valid.
func validateNeeds(kind string, steps []Step, index map[string]int) error {
	for _, step := range steps {
		id := strings.TrimSpace(step.ID)
		if step.needsInvalid {
			return fmt.Errorf("%s %q needs must be a list of %s ids", kind, id, kind)
		}
		seen := make(map[string]struct{}, len(step.Needs))
		for _, need := range step.Needs {
			if _, ok := index[need]; !ok {
				return fmt.Errorf("%s %q needs unknown %s %q", kind, id, kind, need)
			}
			if need == id {
				return fmt.Errorf("%s %q depends on itself", kind, id)
			}
			if _, ok := seen[need]; ok {
				return fmt.Errorf("%s %q repeats %s %q", kind, id, kind, need)
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
