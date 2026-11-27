package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
)

// Registration states of a flow, as `flows` reports them.
const (
	regRegistered    = "registered"
	regPaused        = "paused"
	regNotRegistered = "not-registered"
	// regUnknown is reported when the daemon does not run, so nobody can say.
	regUnknown = "unknown"
)

// flowScheduleDoc is one schedule in the JSON of the flows commands.
type flowScheduleDoc struct {
	Cron     string `json:"cron"`
	Timezone string `json:"timezone,omitempty"`
	Missed   string `json:"missed"`
}

// flowStepDoc is one step of a flow in the JSON of `flows show`.
type flowStepDoc struct {
	ID         string   `json:"id"`
	Needs      []string `json:"needs,omitempty"`
	Run        string   `json:"run"`
	Timeout    string   `json:"timeout,omitempty"`
	Retries    int      `json:"retries,omitempty"`
	ContinueOn bool     `json:"continue_on_error,omitempty"`
	If         string   `json:"if,omitempty"`
}

// flowDoc is what the flows commands know about one pipeline file. Env holds
// variable names only, never values; so does Secrets.
type flowDoc struct {
	Name        string            `json:"name"`
	File        string            `json:"file"`
	Path        string            `json:"path"`
	Valid       bool              `json:"valid"`
	Error       string            `json:"error,omitempty"`
	Pipeline    string            `json:"pipeline,omitempty"`
	Daemon      string            `json:"daemon"`
	Concurrency int               `json:"concurrency,omitempty"`
	Overlap     string            `json:"overlap,omitempty"`
	Jobs        int               `json:"jobs"`
	Schedules   []flowScheduleDoc `json:"schedules"`
	// Hooks are the flow's on.file and on.run, as text (spec 029).
	Hooks   []string      `json:"hooks"`
	Env     []string      `json:"env"`
	Secrets []string      `json:"secrets"`
	Steps   []flowStepDoc `json:"steps"`
}

// describeFlow loads one pipeline file for display. A file that does not load
// is a valid result with Valid false, so one broken flow never hides the rest.
func describeFlow(ctx context.Context, file pipelineFile, reg []api.Pipeline, daemonUp bool) flowDoc {
	d := flowDoc{Name: file.stem, File: file.display, Path: file.path, Daemon: regUnknown,
		Schedules: []flowScheduleDoc{}, Hooks: []string{}, Env: []string{}, Secrets: []string{}, Steps: []flowStepDoc{}}
	if daemonUp {
		d.Daemon = regNotRegistered
		if p, ok := findRegistered(reg, file.path); ok {
			d.Daemon = regRegistered
			if p.Paused {
				d.Daemon = regPaused
			}
		}
	}
	l, err := load(ctx, file)
	if err != nil {
		d.Error = firstLine(err.Error())
		return d
	}
	plan, err := pipeline.Plan(l.def)
	if err != nil {
		d.Error = firstLine(err.Error())
		return d
	}
	d.Valid = true
	d.Pipeline = plan.Name
	d.Concurrency = plan.Concurrency
	d.Overlap = string(l.def.Overlap)
	if d.Overlap == "" {
		d.Overlap = string(pipeline.OverlapQueue)
	}
	d.Jobs = len(l.def.Jobs)
	for _, s := range l.def.Schedules {
		d.Schedules = append(d.Schedules, flowScheduleDoc{Cron: s.Cron, Timezone: s.Timezone, Missed: string(s.Missed)})
	}
	if h := l.def.FileHook; h != nil {
		text := "files change: " + strings.Join(h.Paths, ", ")
		if len(h.Ignore) > 0 {
			text += " (ignoring " + strings.Join(h.Ignore, ", ") + ")"
		}
		d.Hooks = append(d.Hooks, text+", after "+h.Debounce.String())
	}
	for _, h := range l.def.RunHooks {
		status := "succeeded"
		if len(h.Status) > 0 {
			status = strings.Join(h.Status, " or ")
		}
		d.Hooks = append(d.Hooks, "a run of "+h.Flow+" "+status)
	}
	env, secrets := map[string]bool{}, map[string]bool{}
	for _, s := range plan.Graph {
		for k := range s.Env {
			env[k] = true
		}
		for _, k := range s.Secrets {
			secrets[k] = true
		}
		step := flowStepDoc{ID: s.ID, Needs: s.Needs, Run: s.Run, Retries: s.Retries, ContinueOn: s.ContinueOnError}
		if s.Timeout > 0 {
			step.Timeout = s.Timeout.String()
		}
		if s.If != nil {
			step.If = s.If.String()
		}
		d.Steps = append(d.Steps, step)
	}
	d.Env, d.Secrets = sortedKeys(env), sortedKeys(secrets)
	return d
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// daemonFlows reads the registrations when a daemon runs; the flows commands
// work without one.
func (in invocation) daemonFlows(ctx context.Context) ([]api.Pipeline, bool, error) {
	snap, err := in.gather(ctx, 1, false)
	return snap.Pipelines, snap.Status != nil, err
}

// flows is the group of commands that read the pipeline files of the project:
// list, show and graph. None of them runs anything.
func (in invocation) flows(ctx context.Context) int {
	sub, rest := "list", in.args
	if len(in.args) > 0 && !strings.HasPrefix(in.args[0], "-") {
		sub, rest = in.args[0], in.args[1:]
	}
	switch sub {
	case "list":
		return in.flowsList(ctx, rest)
	case "show":
		return in.flowsShow(ctx, rest)
	case "graph":
		return in.flowsGraph(ctx, rest)
	}
	return in.usage()
}

func (in invocation) flowsList(ctx context.Context, args []string) int {
	pos, flags, err := options(args, nil, []string{"json"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	proj, err := project.Open(workingRoot(in.cwd))
	if err != nil {
		return in.fail(err)
	}
	flows, err := proj.Flows()
	if err != nil {
		return in.fail(err)
	}
	reg, up, err := in.daemonFlows(ctx)
	if err != nil {
		return in.fail(err)
	}
	docs := []flowDoc{}
	for _, f := range flows {
		docs = append(docs, describeFlow(ctx, pipelineFile{path: f.Path, display: ".truss/flows/" + f.File, stem: f.Name}, reg, up))
	}
	if flags["json"] != "" {
		return writeJSON(in.stdout, docs)
	}
	if len(docs) == 0 {
		fmt.Fprintln(in.stdout, project.AvailableText(nil))
		return 0
	}
	rows := [][]string{{"FLOW", "STEPS", "SCHEDULES", "DAEMON", "FILE"}}
	for _, d := range docs {
		steps, scheds := fmt.Sprint(len(d.Steps)), fmt.Sprint(len(d.Schedules))
		if !d.Valid {
			steps, scheds = "-", "-"
		}
		file := d.File
		if !d.Valid {
			file += "  INVALID: " + d.Error
		}
		rows = append(rows, []string{d.Name, steps, scheds, d.Daemon, file})
	}
	printColumns(in.stdout, rows)
	if !up {
		fmt.Fprintln(in.stdout, "\nThe daemon is not running, so registration is unknown (start it with: truss daemon start).")
	}
	return 0
}

func (in invocation) flowsShow(ctx context.Context, args []string) int {
	pos, flags, err := options(args, nil, []string{"json", "values"})
	if err != nil || len(pos) < 1 || len(pos) > 2 || (flags["values"] != "" && len(pos) != 2) {
		return in.usage()
	}
	file, err := resolveFile(in.cwd, pos[0])
	if err != nil {
		return in.fail(err)
	}
	if len(pos) == 2 { // one step of the flow (spec 023)
		return in.stepShow(ctx, file, pos[1], flags)
	}
	reg, up, err := in.daemonFlows(ctx)
	if err != nil {
		return in.fail(err)
	}
	d := describeFlow(ctx, file, reg, up)
	if flags["json"] != "" {
		code := writeJSON(in.stdout, d)
		if !d.Valid { // the document says why; the exit code says it too
			return 1
		}
		return code
	}
	if !d.Valid {
		return in.fail(fmt.Errorf("%s: %s", d.File, d.Error))
	}
	line := func(label, text string) { fmt.Fprintf(in.stdout, "%-12s  %s\n", label, clean(text)) }
	list := func(names []string) string {
		if len(names) == 0 {
			return "none"
		}
		return strings.Join(names, ", ")
	}
	line("Flow", d.Pipeline)
	line("File", d.File)
	if root, name := projectOfFile(d.Path); root != "" {
		line("Project", name)
	}
	line("Daemon", d.Daemon)
	line("Concurrency", fmt.Sprint(d.Concurrency))
	line("Overlap", d.Overlap)
	if d.Jobs > 0 {
		line("Jobs", fmt.Sprint(d.Jobs))
	}
	line("Environment", list(d.Env)+" (names only)")
	line("Secrets", list(d.Secrets)+" (names only)")
	if len(d.Schedules) == 0 {
		line("Schedules", "none")
	}
	for _, s := range d.Schedules {
		zone := "local time"
		if s.Timezone != "" {
			zone = s.Timezone
		}
		line("Schedule", fmt.Sprintf("%s (%s, missed: %s)", s.Cron, zone, s.Missed))
	}
	for _, h := range d.Hooks {
		line("Starts when", h)
	}
	fmt.Fprintln(in.stdout)
	rows := [][]string{{"STEP", "NEEDS", "TIMEOUT", "RETRIES", "IF", "RUN"}}
	for _, s := range d.Steps {
		timeout, retries, cond := dash(s.Timeout), "-", dash(s.If)
		if s.Retries > 0 {
			retries = fmt.Sprint(s.Retries)
		}
		rows = append(rows, []string{s.ID, dash(strings.Join(s.Needs, ",")), timeout, retries, cond, runCell(s.Run)})
	}
	for _, l := range columns(rows, screenWidth(in.env)) {
		fmt.Fprintln(in.stdout, l)
	}
	return 0
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// flowsGraph draws the order a flow runs its steps in: one block per stage,
// each step with the steps it waits for. Steps of a stage can run together.
func (in invocation) flowsGraph(ctx context.Context, args []string) int {
	pos, _, err := options(args, nil, nil)
	if err != nil || len(pos) != 1 {
		return in.usage()
	}
	file, err := resolveFile(in.cwd, pos[0])
	if err != nil {
		return in.fail(err)
	}
	l, err := load(ctx, file)
	if err != nil {
		return in.fail(err)
	}
	plan, err := pipeline.Plan(l.def)
	if err != nil {
		return in.fail(err)
	}
	writeStages(in.stdout, plan)
	return 0
}

// stages groups steps by depth: a step is one stage after its latest need.
func stages(plan pipeline.ExecutionPlan) [][]pipeline.Step {
	level := map[string]int{}
	var out [][]pipeline.Step
	// Steps is a valid order, so every need is placed before its step.
	for _, s := range plan.Steps {
		n := 0
		for _, need := range s.Needs {
			n = max(n, level[need]+1)
		}
		level[s.ID] = n
		for len(out) <= n {
			out = append(out, nil)
		}
		out[n] = append(out[n], s)
	}
	return out
}

func writeStages(w io.Writer, plan pipeline.ExecutionPlan) {
	st := stages(plan)
	fmt.Fprintf(w, "%s: %d %s in %d %s, at most %d at once\n", clean(plan.Name),
		len(plan.Graph), plural(len(plan.Graph), "step", "steps"), len(st), plural(len(st), "stage", "stages"), plan.Concurrency)
	for i, group := range st {
		fmt.Fprintf(w, "\nstage %d\n", i+1)
		width := 0
		for _, s := range group {
			width = max(width, utf8.RuneCountInString(clean(s.ID)))
		}
		for _, s := range group {
			id := clean(s.ID)
			if len(s.Needs) == 0 {
				fmt.Fprintf(w, "  [%s]\n", id)
				continue
			}
			fmt.Fprintf(w, "  [%s]%s  <- %s\n", id, strings.Repeat(" ", width-utf8.RuneCountInString(id)), clean(strings.Join(s.Needs, ", ")))
		}
	}
}

// runCell is the first line of a run string for a table cell, marked when the
// command has more lines, so the table never hides that something follows.
// --json has the whole text.
func runCell(run string) string {
	text := strings.TrimSpace(run)
	first := firstLine(text)
	if more := strings.Count(text, "\n"); more > 0 {
		return fmt.Sprintf("%s (+%d %s)", first, more, plural(more, "line", "lines"))
	}
	return first
}
