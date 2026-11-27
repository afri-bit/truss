package pipeline

import (
	"fmt"
	"runtime"
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

// foldEnvNames says that the host treats environment variable names without
// regard to case. Windows does; the other systems do not.
const foldEnvNames = runtime.GOOS == "windows"

// EnvKey returns the form of an environment variable name under which the
// host sees one variable: the name itself, or in upper case on Windows.
func EnvKey(name string) string { return envKey(foldEnvNames, name) }

func envKey(fold bool, name string) string {
	if fold {
		return strings.ToUpper(name)
	}
	return name
}

// overrideEnv returns base with over applied. A variable of base that the
// host would see as the same as one of over is dropped, so the override wins
// whatever the case.
func overrideEnv(base, over map[string]string, fold bool) map[string]string {
	if len(over) == 0 {
		return mergeEnv(base)
	}
	out := make(map[string]string, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for name, value := range over {
		for k := range out {
			if envKey(fold, k) == envKey(fold, name) {
				delete(out, k)
			}
		}
		out[name] = value
	}
	return out
}

// Slice returns a flat pipeline that holds only the selected steps. Each
// step keeps its merged environment, secrets, directory and settings. A need
// outside the slice is dropped, so a step starts as if the steps it needs had
// succeeded. The condition of the job of the named step is dropped: naming a
// step runs it. Every other job in the slice keeps its condition, and the
// step's own condition is kept. p is not changed. The result is meant to be
// run, not registered: it has no schedules, overlap or workdir.
func Slice(p Pipeline, sel Selection) (Pipeline, error) {
	return slice(p, sel, foldEnvNames)
}

// slice is Slice for a host that folds the case of environment names or not.
func slice(p Pipeline, sel Selection, fold bool) (Pipeline, error) {
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

	namedJob := byID[sel.Step].Job
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
				if envKey(fold, secret) == envKey(fold, name) {
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
		if sel.Step != "" && s.Job == namedJob {
			s.JobIf = nil
		}
		s.Env = overrideEnv(s.Env, sel.Env, fold)
		if len(sel.Env) > 0 {
			// --env values are taken as written: "$" is escaped so the
			// expansion of ${NAME} (spec 026) leaves them alone.
			top := make(map[string]string, len(sel.Env))
			for name, value := range sel.Env {
				top[name] = strings.ReplaceAll(value, "$", "$$")
			}
			s.EnvLayers = append(append([]map[string]string(nil), s.EnvLayers...), top)
		}
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

// SetEnv records name=value in env. The later of two names the host sees as
// one variable wins, so an earlier one that differs only in case on Windows
// is dropped.
func SetEnv(env map[string]string, name, value string) { setEnv(env, name, value, foldEnvNames) }

func setEnv(env map[string]string, name, value string, fold bool) {
	for have := range env {
		if envKey(fold, have) == envKey(fold, name) {
			delete(env, have)
		}
	}
	env[name] = value
}
