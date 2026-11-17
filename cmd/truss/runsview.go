package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/afri-bit/truss/internal/api"
)

// filterFetch is how many runs are read when a filter is set, so that a
// filter can reach past the newest few.
const filterFetch = 1000

// defaultRuns is how many runs `truss runs` lists.
const defaultRuns = 30

// knownStatuses are the run statuses --status accepts.
var knownStatuses = []string{api.RunQueued, api.RunRunning, api.RunSucceeded, api.RunFailed, api.RunCancelled, api.RunSkipped}

// pipelines lists the registered pipelines, or shows one of them.
//
//	truss pipelines [list] [--json]
//	truss pipelines show <name> [--json]
func (in invocation) pipelines(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"json"})
	if err != nil {
		return in.usage()
	}
	show := false
	switch {
	case len(pos) == 0:
	case len(pos) == 1 && pos[0] == "list":
	case len(pos) == 2 && pos[0] == "show":
		show = true
	default:
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	list, err := c.Pipelines(ctx)
	if err != nil {
		return in.fail(err)
	}
	if show {
		return in.pipelineShow(ctx, c, list, pos[1], flags["json"] != "")
	}
	if flags["json"] != "" {
		return writeJSON(in.stdout, list)
	}
	if len(list) == 0 {
		fmt.Fprintln(in.stdout, "no pipelines registered (add one with: truss register <path>)")
		return 0
	}
	w := tabwriter.NewWriter(in.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tNEXT RUN\tLAST RUN\tFILE")
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", p.Name, pipelineState(p), nextRun(p), lastRun(p), p.Path)
	}
	_ = w.Flush()
	return 0
}

// pipelineShow prints one registered pipeline: its file, project, state,
// schedules, steps and latest runs. Either of its names finds it.
func (in invocation) pipelineShow(ctx context.Context, c *api.Client, list []api.Pipeline, name string, asJSON bool) int {
	var p *api.Pipeline
	for i := range list {
		if list[i].Name == name || list[i].RegisteredName == name {
			p = &list[i]
		}
	}
	if p == nil {
		return in.fail(fmt.Errorf("no pipeline named %q is registered (list them with: truss pipelines)", name))
	}
	if asJSON {
		return writeJSON(in.stdout, p)
	}
	runs, err := c.Runs(ctx, p.Name, 5)
	if err != nil {
		return in.fail(err)
	}
	now := time.Now()
	line := func(label, text string) { fmt.Fprintf(in.stdout, "%-12s  %s\n", label, text) }
	line("Pipeline", p.Name)
	if p.RegisteredName != "" {
		line("Registered as", p.RegisteredName)
	}
	line("State", pipelineState(*p))
	line("File", p.Path)
	if _, project := projectOfFile(p.Path); project != "" {
		line("Project", project)
	}
	if !p.Valid {
		line("Problem", firstLine(p.Error))
	} else {
		line("Concurrency", fmt.Sprint(max(p.Concurrency, 1)))
		line("Overlap", dash(p.Overlap))
	}
	line("Registered", local(&p.RegisteredAt))
	line("Active runs", fmt.Sprint(p.ActiveRuns))
	if len(p.Schedules) == 0 {
		line("Schedules", "none")
	}
	for _, s := range p.Schedules {
		zone := "local time"
		if s.Timezone != "" {
			zone = s.Timezone
		}
		line("Schedule", fmt.Sprintf("%s (%s, missed: %s), next %s", s.Cron, zone, s.Missed, local(s.Next)))
	}
	if len(p.Steps) > 0 {
		fmt.Fprintln(in.stdout, "\nSTEPS")
		for _, s := range p.Steps {
			text := "  " + s.ID
			if len(s.Needs) > 0 {
				text += "  <- " + strings.Join(s.Needs, ", ")
			}
			fmt.Fprintln(in.stdout, text)
		}
	}
	if len(runs) > 0 {
		fmt.Fprintln(in.stdout, "\nLATEST RUNS")
		rows := [][]string{{"RUN", "STATUS", "TRIGGER", "WHEN", "TOOK"}}
		for _, r := range runs {
			rows = append(rows, []string{r.ID, r.Status, r.Trigger, age(now, r.CreatedAt), elapsed(r.StartedAt, r.FinishedAt, now)})
		}
		for _, l := range columns(rows, noLimit) {
			fmt.Fprintln(in.stdout, "  "+l)
		}
	}
	return 0
}

// runs lists runs, newest first, or shows one run.
//
//	truss runs [<pipeline>] [--status <s>] [--active] [--limit <n>] [--json]
//	truss runs show <run-id|last> [--pipeline <name>] [--json]
//
// A pipeline may be named "show"; `truss runs show` alone still lists its
// runs, and only `runs show <something>` shows a run.
func (in invocation) runs(ctx context.Context) int {
	if len(in.args) >= 2 && in.args[0] == "show" {
		return in.runShow(ctx, in.args[1:])
	}
	pos, flags, err := options(in.args, []string{"status", "limit"}, []string{"json", "active"})
	if err != nil || len(pos) > 1 {
		return in.usage()
	}
	limit := defaultRuns
	if text, ok := flags["limit"]; ok {
		if limit, err = positive(text); err != nil {
			return in.fail(fmt.Errorf("--limit must be a whole number of at least 1, got %q", text))
		}
	}
	status := flags["status"]
	if status != "" && flags["active"] != "" {
		return in.fail(errors.New("--status and --active cannot be used together"))
	}
	if _, ok := flags["status"]; ok && !contains(knownStatuses, status) {
		return in.fail(fmt.Errorf("--status must be one of %s, got %q", strings.Join(knownStatuses, ", "), status))
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	filtered := status != "" || flags["active"] != ""
	fetch := limit
	if filtered {
		fetch = filterFetch
	}
	list, err := c.Runs(ctx, strings.Join(pos, ""), fetch)
	if err != nil {
		return in.fail(err)
	}
	if filtered {
		kept := list[:0:0]
		for _, r := range list {
			if (status != "" && r.Status == status) || (flags["active"] != "" && r.Active()) {
				kept = append(kept, r)
			}
		}
		list = kept[:min(len(kept), limit)]
	}
	if flags["json"] != "" {
		if list == nil {
			list = []api.Run{}
		}
		return writeJSON(in.stdout, list)
	}
	if len(list) == 0 {
		if filtered {
			fmt.Fprintln(in.stdout, "no runs match")
		} else {
			fmt.Fprintln(in.stdout, "no runs yet")
		}
		return 0
	}
	w := tabwriter.NewWriter(in.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RUN\tPIPELINE\tSTATUS\tTRIGGER\tCREATED\tDURATION\tNOTE")
	for _, r := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Pipeline, r.Status, r.Trigger, local(&r.CreatedAt), duration(r), r.Reason)
	}
	_ = w.Flush()
	return 0
}

func positive(text string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(text, "%d", &n); err != nil || n < 1 || fmt.Sprint(n) != text {
		return 0, errors.New("not a positive number")
	}
	return n, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// runShow prints the state of one run: its status, its steps with their
// exit codes and times, and a timeline of when each step ran. The id "last"
// is the newest run, of one pipeline with --pipeline.
func (in invocation) runShow(ctx context.Context, args []string) int {
	pos, flags, err := options(args, []string{"pipeline"}, []string{"json"})
	if err != nil || len(pos) != 1 {
		return in.usage()
	}
	if _, ok := flags["pipeline"]; ok && pos[0] != "last" {
		return in.fail(errors.New("--pipeline only goes with the id \"last\""))
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	id := pos[0]
	if id == "last" {
		list, err := c.Runs(ctx, flags["pipeline"], 1)
		if err != nil {
			return in.fail(err)
		}
		if len(list) == 0 {
			return in.fail(errors.New("there are no runs yet"))
		}
		id = list[0].ID
	}
	run, err := c.Run(ctx, id)
	if err != nil {
		var status *api.StatusError
		if errors.As(err, &status) && status.Code == http.StatusNotFound {
			return in.fail(fmt.Errorf("no run %q (list runs with: truss runs)", id))
		}
		return in.fail(err)
	}
	if flags["json"] != "" {
		return writeJSON(in.stdout, run)
	}
	writeRun(in.stdout, run, time.Now(), screenWidth(in.env))
	return 0
}

// writeRun prints one run for `runs show`.
func writeRun(w interface{ Write([]byte) (int, error) }, r api.Run, now time.Time, width int) {
	line := func(label, text string) { fmt.Fprintf(w, "%-9s  %s\n", label, text) }
	line("Run", r.ID)
	line("Pipeline", r.Pipeline)
	status := r.Status
	if r.Reason != "" {
		status += ": " + firstLine(r.Reason)
	}
	line("Status", status)
	line("Trigger", r.Trigger)
	line("Created", fmt.Sprintf("%s (%s)", local(&r.CreatedAt), age(now, r.CreatedAt)))
	if r.StartedAt != nil {
		line("Took", elapsed(r.StartedAt, r.FinishedAt, now))
	}
	counts := map[string]int{}
	for _, s := range r.Steps {
		counts[s.Status]++
	}
	var parts []string
	for _, st := range []string{api.StepSucceeded, api.StepFailed, api.StepRunning, api.StepPending, api.StepStopped, api.StepCancelled, api.StepSkipped} {
		if counts[st] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[st], st))
		}
	}
	line("Steps", fmt.Sprintf("%d (%s)", len(r.Steps), strings.Join(parts, ", ")))
	line("Log", "truss logs "+r.ID)
	if len(r.Steps) == 0 {
		return
	}
	fmt.Fprintln(w)
	cells := min(max(width-62, 12), 40)
	end := now
	if r.FinishedAt != nil {
		end = *r.FinishedAt
	}
	rows := [][]string{{"STEP", "STATUS", "EXIT", "TOOK", "TRY", "TIMELINE"}}
	for _, s := range r.Steps {
		exit, try := "-", "-"
		if s.ExitCode != nil {
			exit = fmt.Sprint(*s.ExitCode)
		}
		if s.Attempts > 1 {
			try = fmt.Sprint(s.Attempts)
		}
		timeline := ""
		if r.StartedAt != nil {
			timeline = span(s.StartedAt, s.FinishedAt, *r.StartedAt, end, cells)
		}
		rows = append(rows, []string{s.ID, s.Status, exit, elapsed(s.StartedAt, s.FinishedAt, now), try, timeline})
	}
	for _, l := range columns(rows, noLimit) {
		fmt.Fprintln(w, l)
	}
	var notes []string
	for _, s := range r.Steps {
		switch {
		case s.Error != "":
			notes = append(notes, fmt.Sprintf("  %s: %s", s.ID, firstLine(s.Error)))
		case s.Reason != "":
			notes = append(notes, fmt.Sprintf("  %s: %s", s.ID, firstLine(s.Reason)))
		}
	}
	if len(notes) > 0 {
		fmt.Fprintln(w, "\nNOTES")
		for _, n := range notes {
			fmt.Fprintln(w, n)
		}
	}
}
