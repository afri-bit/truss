package pipeline

import (
	"fmt"
	"sort"
	"strings"
)

// Mode says which steps around the named step a slice holds.
type Mode int

const (
	// Only is the named step alone.
	Only Mode = iota
	// WithNeeds is the named step and every step it needs, directly or not.
	WithNeeds
	// From is the named step and every step that needs it, directly or not.
	From
)

// Selection names the part of a pipeline a debug run executes (spec 023).
type Selection struct {
	// Step is the id of a step as the plan names it: "job/step" in a job. An
	// empty Step selects every step and keeps the conditions of jobs: the
	// result is the whole pipeline with Env applied.
	Step string
	Mode Mode
	// Env replaces or adds environment variables of every step in the slice,
	// above every layer of the file.
	Env map[string]string
}

// Slice returns a flat pipeline that holds only the selected steps. Each
// step keeps its merged environment, secrets, directory and settings. A need
// outside the slice is dropped, so a step starts as if the steps it needs had
// succeeded. The condition of a job is dropped: naming a step runs it. The
// step's own condition is kept. p is not changed.
func Slice(p Pipeline, sel Selection) (Pipeline, error) {
	plan, err := Plan(p)
	if err != nil {
		return Pipeline{}, err
	}
	byID := make(map[string]Step, len(plan.Graph))
	for _, s := range plan.Graph {
		byID[s.ID] = s
	}
	if _, ok := byID[sel.Step]; sel.Step != "" && !ok {
		known := make([]string, len(plan.Graph))
		for i, s := range plan.Graph {
			known[i] = s.ID
		}
		return Pipeline{}, fmt.Errorf("no step %q in flow %q; its steps: %s", sel.Step, plan.Name, strings.Join(known, ", "))
	}

	in := map[string]bool{sel.Step: true}
	if sel.Step == "" {
		for _, s := range plan.Graph {
			in[s.ID] = true
		}
		sel.Mode = Only
	}
	for grew := true; grew && sel.Mode != Only; {
		grew = false
		for _, s := range plan.Graph {
			switch sel.Mode {
			case WithNeeds:
				if in[s.ID] {
					for _, need := range s.Needs {
						if !in[need] {
							in[need], grew = true, true
						}
					}
				}
			case From:
				if in[s.ID] {
					continue
				}
				for _, need := range s.Needs {
					if in[need] {
						in[s.ID], grew = true, true
						break
					}
				}
			}
		}
	}

	names := make([]string, 0, len(sel.Env))
	for name := range sel.Env {
		names = append(names, name)
	}
	sort.Strings(names)

	var steps []Step
	for _, s := range plan.Graph {
		if !in[s.ID] {
			continue
		}
		for _, name := range names {
			for _, secret := range s.Secrets {
				if secret == name {
					return Pipeline{}, fmt.Errorf("cannot override %s: step %q reads it as a secret; use truss secret set %s", name, s.ID, name)
				}
			}
		}
		var needs []string
		for _, need := range s.Needs {
			if in[need] {
				needs = append(needs, need)
			}
		}
		s.Needs = needs
		if sel.Step != "" {
			s.JobIf = nil
		}
		s.Env = mergeEnv(s.Env, sel.Env)
		steps = append(steps, s)
	}
	return Pipeline{Version: "1", Name: plan.Name, Steps: steps, Concurrency: plan.Concurrency}, nil
}

// NeededBy returns the ids of the steps that list id in their needs, in
// definition order.
func (p ExecutionPlan) NeededBy(id string) []string {
	var out []string
	for _, s := range p.Graph {
		for _, need := range s.Needs {
			if need == id {
				out = append(out, s.ID)
				break
			}
		}
	}
	return out
}
