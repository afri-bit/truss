package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
	"github.com/afri-bit/truss/internal/secrets"
)

// The step view of `flows show <flow> <step>` (spec 023). It reads the flow
// and the environment and changes nothing.

// Kinds of environment row, as the JSON names them.
const (
	kindDeclared   = "declared"
	kindSecret     = "secret"
	kindTruss      = "truss"
	kindReferenced = "referenced"
)

// States of an environment row, as the JSON names them. The text form says
// "not set" and "each run".
const (
	stateSet     = "set"
	stateNotSet  = "not-set"
	stateMissing = "missing"
	// stateEachRun is a built-in variable whose value each run sets anew,
	// such as the run id (spec 026).
	stateEachRun = "each-run"
)

// masked is what the screen shows instead of a value it must not print.
const masked = "***"

// envRow is one variable a step can use.
type envRow struct {
	Name   string  `json:"name"`
	Source string  `json:"source"`
	Kind   string  `json:"kind"`
	State  string  `json:"state"`
	Value  *string `json:"value,omitempty"`
}

// stepInfo is the step itself in the JSON of the step view. RunDir is the
// directory the step starts in, worked out as a run works it out; DirError
// says why it cannot be, and then RunDir is empty.
type stepInfo struct {
	ID         string   `json:"id"`
	Job        string   `json:"job,omitempty"`
	Run        string   `json:"run"`
	Dir        string   `json:"dir,omitempty"`
	RunDir     string   `json:"run_dir"`
	DirError   string   `json:"dir_error,omitempty"`
	Needs      []string `json:"needs"`
	NeededBy   []string `json:"needed_by"`
	Timeout    string   `json:"timeout,omitempty"`
	Retries    int      `json:"retries,omitempty"`
	RetryDelay string   `json:"retry_delay,omitempty"`
	ContinueOn bool     `json:"continue_on_error,omitempty"`
	If         string   `json:"if,omitempty"`
	JobIf      string   `json:"job_if,omitempty"`
}

// stepDoc is the JSON document of the step view.
type stepDoc struct {
	Flow        string   `json:"flow"`
	File        string   `json:"file"`
	Project     string   `json:"project,omitempty"`
	Step        stepInfo `json:"step"`
	Environment []envRow `json:"environment"`
}

// sensitiveName matches the names whose values are never printed: a screen
// can end up in a screenshot (ADR 0020). It is a safeguard against accidents,
// not a defence against a file that prints a variable itself.
var sensitiveName = regexp.MustCompile(`(?i)TOKEN|SECRET|PASS|CREDENTIAL|PRIVATE|AUTH|KEY|DSN|COOKIE|SESSION|JWT|BEARER|WEBHOOK`)

// readPattern finds the forms in which a command reads a variable: $env:NAME
// (any case of "env"), ${NAME, $NAME and %NAME%. Each name is a whole
// identifier, so $Home is "Home" and not "H".
var readPattern = regexp.MustCompile(`(?i:\$env:)([A-Za-z_][A-Za-z0-9_]*)|\$\{([A-Za-z_][A-Za-z0-9_]*)|\$([A-Za-z_][A-Za-z0-9_]*)|%([A-Za-z_][A-Za-z0-9_]*)%`)

// Forms in which a command sets a variable itself: NAME=value after a
// space, a separator or a quote (also cmd's set "NAME=x"), $env:NAME = value,
// and for NAME in.
var (
	assignPattern   = regexp.MustCompile(`(?m)(?:^|[\s;&|("])([A-Za-z_][A-Za-z0-9_]*)=`)
	psAssignPattern = regexp.MustCompile(`(?i:\$env:)([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	forPattern      = regexp.MustCompile(`\bfor\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
)

// references returns, sorted, the variable names a command reads and does not
// set itself. A name counts when it is upper case, digits and _, which leaves
// out the loop variables and locals of most scripts; %NAME% needs two
// characters, so date formats such as %Y%m do not count. The scan is linear in
// the length of the command.
func references(run string) []string {
	assigned := map[string]bool{}
	for _, re := range []*regexp.Regexp{assignPattern, psAssignPattern, forPattern} {
		for _, m := range re.FindAllStringSubmatch(run, -1) {
			assigned[m[1]] = true
		}
	}
	found := map[string]bool{}
	for _, m := range readPattern.FindAllStringSubmatch(run, -1) {
		for i, name := range m[1:] {
			if name == "" || name == "_" || name != strings.ToUpper(name) || assigned[name] {
				continue
			}
			if i == 3 && len(name) < 2 { // %X%
				continue
			}
			found[name] = true
		}
	}
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// stepView builds what `flows show <flow> <step>` prints. The secrets store is
// opened only when the step declares secrets or showValues needs the stored
// values, so a step without them needs no TRUSS home.
func (in invocation) stepView(file pipelineFile, def pipeline.Pipeline, id string, showValues bool) (stepDoc, error) {
	plan, err := pipeline.Plan(def)
	if err != nil {
		return stepDoc{}, err
	}
	var step pipeline.Step
	found := false
	var known []string
	for _, s := range plan.Graph {
		known = append(known, s.ID)
		if s.ID == id {
			step, found = s, true
		}
	}
	if !found {
		return stepDoc{}, fmt.Errorf("no step %q in flow %q; its steps: %s", id, plan.Name, strings.Join(known, ", "))
	}
	root := workingRoot(in.cwd)
	st, err := project.StateFor(root, file.stem)
	if err != nil {
		return stepDoc{}, err
	}

	d := stepDoc{Flow: plan.Name, File: file.display, Environment: []envRow{}}
	if _, name := projectOfFile(file.path); name != "" {
		d.Project = name
	}
	d.Step = stepInfo{ID: step.ID, Job: step.Job, Run: step.Run, Dir: step.Dir,
		Needs: nonNil(step.Needs), NeededBy: nonNil(plan.NeededBy(step.ID)),
		Retries: step.Retries, ContinueOn: step.ContinueOnError}
	stepDir, _ := pipeline.Expand(step.Dir, func(name string) string { v, _ := envLookup(in.env)(name); return v })
	if dir, err := engine.StepDir(root, stepDir); err != nil {
		d.Step.DirError = err.Error()
	} else {
		d.Step.RunDir = dir
	}
	if step.Timeout > 0 {
		d.Step.Timeout = step.Timeout.String()
	}
	if step.Retries > 0 && step.RetryDelay > 0 {
		d.Step.RetryDelay = step.RetryDelay.String()
	}
	if step.If != nil {
		d.Step.If = step.If.String()
	}
	if step.JobIf != nil {
		d.Step.JobIf = step.JobIf.String()
	}

	var stored []string // the values of the stored secrets; read only for --values
	var storedNames map[string]bool
	if len(step.Secrets) > 0 || showValues {
		h, err := home.Resolve(in.env)
		if err != nil {
			return stepDoc{}, err
		}
		store := secrets.Open(h.Secrets())
		names, err := store.Names()
		if err != nil {
			return stepDoc{}, err
		}
		storedNames = map[string]bool{}
		for _, n := range names {
			storedNames[n] = true // the store and the engine match secret names exactly
		}
		if showValues {
			all, err := store.Load()
			if err != nil {
				return stepDoc{}, err
			}
			for _, v := range all {
				if v != "" {
					stored = append(stored, v)
				}
			}
		}
	}
	// holdsSecret reports whether a value contains a stored secret, which a
	// screen must not print even as a part of a longer value.
	holdsSecret := func(v string) bool {
		for _, s := range stored {
			if strings.Contains(v, s) {
				return true
			}
		}
		return false
	}

	// What a step receives, later winning: the base environment of the run
	// (the TRUSS variables), then the file's variables, then the secrets. A
	// name appears once, in the row of the layer that wins.
	isSecret := map[string]bool{}
	for _, name := range step.Secrets {
		isSecret[pipeline.EnvKey(name)] = true
	}
	taken := map[string]bool{}
	for _, name := range sortedStrings(step.Env) {
		if isSecret[pipeline.EnvKey(name)] {
			continue
		}
		taken[pipeline.EnvKey(name)] = true
		v := step.Env[name]
		if sensitiveName.MatchString(name) || holdsSecret(v) {
			v = masked
		}
		d.Environment = append(d.Environment, envRow{Name: name, Source: step.EnvSource[name], Kind: kindDeclared, State: stateSet, Value: &v})
	}
	for _, name := range step.Secrets {
		taken[pipeline.EnvKey(name)] = true
		row := envRow{Name: name, Source: "secret store", Kind: kindSecret, State: stateMissing}
		if storedNames[name] {
			v := masked
			row.State, row.Value = stateSet, &v
		}
		d.Environment = append(d.Environment, row)
	}
	for _, t := range []struct{ name, value string }{{"TRUSS_DATA_DIR", st.Data}, {"TRUSS_CACHE_DIR", st.Cache}} {
		if taken[pipeline.EnvKey(t.name)] {
			continue
		}
		taken[pipeline.EnvKey(t.name)] = true
		v := t.value
		d.Environment = append(d.Environment, envRow{Name: t.name, Source: "truss run", Kind: kindTruss, State: stateSet, Value: &v})
	}
	// The built-in variables of every run (spec 026): those known now with
	// their value, the rest as set by each run.
	stepID := step.ID
	if step.Job != "" {
		stepID = strings.TrimPrefix(stepID, step.Job+"/")
	}
	builtins := []struct{ name, value string }{
		{"TRUSS_FLOW", plan.Name}, {"TRUSS_PIPELINE", plan.Name}, {"TRUSS_STEP", stepID},
		{"TRUSS_JOB", step.Job}, {"TRUSS_PROJECT_DIR", root}, {"TRUSS_OS", runtime.GOOS}, {"TRUSS_ARCH", runtime.GOARCH},
	}
	for _, t := range builtins {
		if t.value == "" || taken[pipeline.EnvKey(t.name)] {
			continue
		}
		taken[pipeline.EnvKey(t.name)] = true
		v := t.value
		d.Environment = append(d.Environment, envRow{Name: t.name, Source: "truss", Kind: kindTruss, State: stateSet, Value: &v})
	}
	for _, name := range []string{"TRUSS_RUN_ID", "TRUSS_TRIGGER", "TRUSS_ATTEMPT", "TRUSS_TEMP_DIR"} {
		if taken[pipeline.EnvKey(name)] {
			continue
		}
		taken[pipeline.EnvKey(name)] = true
		d.Environment = append(d.Environment, envRow{Name: name, Source: "truss", Kind: kindTruss, State: stateEachRun})
	}

	hostEnv := envLookup(in.env)
	for _, name := range references(step.Run) {
		if taken[pipeline.EnvKey(name)] {
			continue
		}
		row := envRow{Name: name, Source: "host environment", Kind: kindReferenced, State: stateNotSet}
		if v, ok := hostEnv(name); ok {
			row.State = stateSet
			switch {
			case !showValues:
			case sensitiveName.MatchString(name) || holdsSecret(v):
				m := masked
				row.Value = &m
			default:
				row.Value = &v
			}
		}
		d.Environment = append(d.Environment, row)
	}
	return d, nil
}

// envLookup returns a function that reads a variable of an environment list.
// The last entry wins, as when a process starts, and names are compared the
// way the host does.
func envLookup(env []string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := "", false
		for _, kv := range env {
			if k, v, has := strings.Cut(kv, "="); has && pipeline.EnvKey(k) == pipeline.EnvKey(name) {
				value, ok = v, true
			}
		}
		return value, ok
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func sortedStrings(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// stepShow is `truss flows show <flow> <step>`.
func (in invocation) stepShow(ctx context.Context, file pipelineFile, id string, flags map[string]string) int {
	l, err := load(ctx, file)
	if err != nil {
		return in.fail(fmt.Errorf("%s: %s", file.display, clean(firstLine(err.Error()))))
	}
	d, err := in.stepView(file, l.def, id, flags["values"] != "")
	if err != nil {
		return in.fail(fmt.Errorf("%s", clean(err.Error())))
	}
	if flags["json"] != "" {
		return writeJSON(in.stdout, d)
	}
	in.writeStep(d)
	return 0
}

// writeStep prints the step view as text.
func (in invocation) writeStep(d stepDoc) {
	w := in.stdout
	line := func(label, text string) { fmt.Fprintf(w, "%-12s  %s\n", label, clean(text)) }
	list := func(names []string) string {
		if len(names) == 0 {
			return "-"
		}
		return strings.Join(names, ", ")
	}
	s := d.Step
	line("Flow", d.Flow)
	line("File", d.File)
	if d.Project != "" {
		line("Project", d.Project)
	}
	line("Step", s.ID)
	if s.Job != "" {
		line("Job", s.Job)
	}
	line("Needs", list(s.Needs))
	line("Needed by", list(s.NeededBy))
	switch {
	case s.DirError != "":
		line("Directory", "cannot be used: "+s.DirError)
	case s.Dir != "":
		line("Directory", s.RunDir+" (dir: "+s.Dir+")")
	default:
		line("Directory", s.RunDir)
	}
	line("Timeout", dash(s.Timeout))
	line("Retries", fmt.Sprint(s.Retries))
	if s.RetryDelay != "" {
		line("Retry delay", s.RetryDelay)
	}
	if s.ContinueOn {
		line("On failure", "the run goes on (continue-on-error)")
	}
	line("If", dash(s.If))
	if s.JobIf != "" {
		line("Job if", s.JobIf)
	}
	fmt.Fprintln(w, "Command")
	for _, l := range strings.Split(strings.TrimRight(s.Run, "\r\n"), "\n") {
		fmt.Fprintf(w, "  %s\n", clean(strings.TrimRight(l, "\r")))
	}
	fmt.Fprintln(w)
	root := workingRoot(in.cwd)
	rows := [][]string{{"NAME", "SOURCE", "STATE", "VALUE"}}
	for _, r := range d.Environment {
		state, value := strings.ReplaceAll(r.State, "-", " "), "-"
		if r.Value != nil {
			value = *r.Value
			if r.Kind == kindTruss { // a path of the project, shown from its root
				if rel, err := filepath.Rel(root, value); err == nil {
					value = rel
				}
			}
		}
		rows = append(rows, []string{r.Name, sourceLabel(r.Source), state, value})
	}
	for _, l := range columns(rows, screenWidth(in.env)) {
		fmt.Fprintln(w, clean(l))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Host environment rows come from reading the command text; a script can read more.")
	fmt.Fprintln(w, "Values of host variables are shown with --values; secrets and token-like names never.")
}

// sourceLabel is the word the table shows for where a variable comes from.
// The flow-level env keeps the source "pipeline" in --json, a released
// contract; the table says "flow", the word TRUSS uses since v0.3.0.
func sourceLabel(source string) string {
	if source == "pipeline" {
		return "flow"
	}
	return source
}
