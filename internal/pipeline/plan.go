package pipeline

import "fmt"

// Plan turns a validated pipeline into an execution plan.
//
// Jobs are flattened: each step of a job becomes a step named "job/step" that
// needs the step before it, and the first step of a job needs the last step
// of every job that job needs. Environment and working directory (step, job,
// then pipeline workdir) are resolved here, so the engine only sees steps.
//
// In Steps a step is placed only after every step it needs. When several
// steps could be placed next, the one written earliest in the definition goes
// first, so a definition without needs keeps definition order.
func Plan(p Pipeline) (ExecutionPlan, error) {
	graph := flatten(p)
	steps := make([]Step, 0, len(graph))
	placed := make(map[string]bool, len(graph))
	used := make([]bool, len(graph))
	for len(steps) < len(graph) {
		next := -1
		for i, step := range graph {
			if !used[i] && ready(step, placed) {
				next = i
				break
			}
		}
		if next < 0 {
			return ExecutionPlan{}, fmt.Errorf("cannot plan steps: no step is ready to run")
		}
		used[next] = true
		placed[graph[next].ID] = true
		steps = append(steps, graph[next])
	}
	return ExecutionPlan{Name: p.Name, Steps: steps, Graph: graph, Concurrency: max(p.Concurrency, 1)}, nil
}

// flatten returns every step of p in definition order, with jobs resolved
// into step ids, needs, environment and working directory.
func flatten(p Pipeline) []Step {
	if len(p.Jobs) == 0 {
		steps := make([]Step, len(p.Steps))
		for i, s := range p.Steps {
			s.EnvSource = envSource(layer{"project", p.ProjectEnv}, layer{"pipeline", p.Env}, layer{"step", s.Env})
			// A step of a debug slice (Slice) is already planned and keeps its
			// layers; it comes with no flow or project env of its own.
			if s.EnvLayers == nil {
				s.EnvLayers = layers(p.ProjectEnv, p.Env, s.Env)
			}
			s.Env = mergeEnv(p.ProjectEnv, p.Env, s.Env)
			s.Secrets = mergeNames(p.Secrets, s.Secrets)
			if s.Dir == "" {
				s.Dir = p.Workdir
			}
			steps[i] = s
		}
		return steps
	}
	last := make(map[string]string, len(p.Jobs))
	for _, job := range p.Jobs {
		if n := len(job.Steps); n > 0 {
			last[job.ID] = joinID(job.ID, job.Steps[n-1].ID)
		}
	}
	var steps []Step
	for _, job := range p.Jobs {
		var needs []string
		for _, need := range job.Needs {
			needs = append(needs, last[need])
		}
		for i, s := range job.Steps {
			s.ID = joinID(job.ID, s.ID)
			s.Job, s.JobFirst, s.JobIf = job.ID, i == 0, job.If
			s.Needs = needs
			s.EnvSource = envSource(layer{"project", p.ProjectEnv}, layer{"pipeline", p.Env}, layer{"job " + job.ID, job.Env}, layer{"step", s.Env})
			s.EnvLayers = layers(p.ProjectEnv, p.Env, job.Env, s.Env)
			s.Env = mergeEnv(p.ProjectEnv, p.Env, job.Env, s.Env)
			s.Secrets = mergeNames(p.Secrets, job.Secrets, s.Secrets)
			// The nearest setting wins: step, job, then pipeline workdir.
			if s.Dir == "" {
				s.Dir = job.Dir
			}
			if s.Dir == "" {
				s.Dir = p.Workdir
			}
			steps = append(steps, s)
			needs = []string{s.ID}
		}
	}
	return steps
}

// layer is one level of environment variables and the name it is shown under.
type layer struct {
	label string
	env   map[string]string
}

// envSource returns, for every variable in the layers, the label of the
// layer that wins (the last one that sets it). It returns nil when no layer
// sets anything.
func envSource(layers ...layer) map[string]string {
	var out map[string]string
	for _, l := range layers {
		for k := range l.env {
			if out == nil {
				out = make(map[string]string)
			}
			out[k] = l.label
		}
	}
	return out
}

// mergeEnv returns one map holding every layer; later layers win.
func mergeEnv(layers ...map[string]string) map[string]string {
	var out map[string]string
	for _, layer := range layers {
		for k, v := range layer {
			if out == nil {
				out = make(map[string]string)
			}
			out[k] = v
		}
	}
	return out
}

// mergeNames returns the names of every layer, in order, without repeats.
func mergeNames(layers ...[]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, layer := range layers {
		for _, n := range layer {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
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

// layers keeps the non-empty variable layers, lowest first.
func layers(all ...map[string]string) []map[string]string {
	var out []map[string]string
	for _, l := range all {
		if len(l) > 0 {
			out = append(out, l)
		}
	}
	return out
}
