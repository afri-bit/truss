package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/project"
)

// noLimit is the width of a table that is never shortened, so that output
// meant for pipes and scripts keeps every character.
const noLimit = 1 << 20

// refreshEvery is how often the live dashboard draws again.
const refreshEvery = 2 * time.Second

// snapshot is what the overview commands show: the daemon, if one answers,
// and what it knows. Every field after Now is empty when no daemon runs.
type snapshot struct {
	Now       time.Time
	Status    *api.Status
	Pipelines []api.Pipeline
	Runs      []api.Run
	Metrics   *api.Metrics
}

// gather reads the daemon once. A daemon that is not running is not an error:
// the snapshot is simply empty. Metrics are read only when asked for, and a
// daemon that cannot give them leaves them out.
func (in invocation) gather(ctx context.Context, runs int, metrics bool) (snapshot, error) {
	snap := snapshot{Now: time.Now()}
	c, err := in.dial(ctx)
	if errors.Is(err, api.ErrNotRunning) {
		return snap, nil
	}
	if err != nil {
		return snap, err
	}
	st, err := c.Status(ctx)
	if err != nil {
		return snap, err
	}
	snap.Status = &st
	if snap.Pipelines, err = c.Pipelines(ctx); err != nil {
		return snap, err
	}
	if snap.Runs, err = c.Runs(ctx, "", runs); err != nil {
		return snap, err
	}
	if metrics {
		if m, err := c.Metrics(ctx); err == nil && m.Supported {
			snap.Metrics = &m
		}
	}
	return snap, nil
}

// projectDoc describes the project of the working directory.
type projectDoc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Root        string `json:"root"`
	Flows       int    `json:"flows"`
}

// currentProject reads the project of the working directory. It returns nil
// and no error when there is none; any other problem is an error.
func (in invocation) currentProject() (*projectDoc, []project.Flow, error) {
	proj, err := project.Open(workingRoot(in.cwd))
	if errors.Is(err, project.ErrNoProject) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	flows, err := proj.Flows()
	if err != nil {
		return nil, nil, err
	}
	return &projectDoc{Name: proj.Name, Description: proj.Description, Root: proj.Root, Flows: len(flows)}, flows, nil
}

// projectOfFile names the project a pipeline file belongs to: the folder that
// holds truss.yaml above .truss/flows/. It returns empty strings for a file
// that is not inside a project's flows folder.
func projectOfFile(path string) (root, name string) {
	flows := filepath.Dir(path)
	truss := filepath.Dir(flows)
	if filepath.Base(flows) != "flows" || filepath.Base(truss) != ".truss" {
		return "", ""
	}
	root = filepath.Dir(truss)
	proj, err := project.Open(root)
	if err != nil {
		return "", ""
	}
	return root, proj.Name
}

func writeJSON(w io.Writer, v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return 1
	}
	_, _ = fmt.Fprintf(w, "%s\n", data)
	return 0
}

func printColumns(w io.Writer, rows [][]string) {
	for _, l := range columns(rows, noLimit) {
		fmt.Fprintln(w, l)
	}
}

// uptime is how long the daemon has been up.
func uptime(snap snapshot) string {
	return short(snap.Now.Sub(snap.Status.StartedAt))
}

type flowCounts struct {
	Registered int `json:"registered"`
	Ready      int `json:"ready"`
	Running    int `json:"running"`
	Paused     int `json:"paused"`
	Invalid    int `json:"invalid"`
}

func countFlows(list []api.Pipeline) flowCounts {
	c := flowCounts{Registered: len(list)}
	for _, p := range list {
		switch pipelineState(p) {
		case "invalid":
			c.Invalid++
		case "running":
			c.Running++
		case "paused":
			c.Paused++
		default:
			c.Ready++
		}
	}
	return c
}

// statusDoc is the JSON of `truss status --json`.
type statusDoc struct {
	Version string      `json:"version"`
	Project *projectDoc `json:"project"`
	// Daemon is null when no daemon runs.
	Daemon     *api.Status `json:"daemon"`
	DesktopURL *string     `json:"desktop_url"`
	// ProjectError and DaemonError say why Project or Daemon is null when the
	// cause is a problem rather than absence.
	ProjectError string     `json:"project_error,omitempty"`
	DaemonError  string     `json:"daemon_error,omitempty"`
	Pipelines    flowCounts `json:"pipelines"`
	RecentRuns   []api.Run  `json:"recent_runs"`
	Home         string     `json:"home"`
}

// status prints the one-screen answer to "is everything fine?": the project,
// the daemon, the flows and the latest run. It exits 0 whether or not a daemon
// runs; `truss daemon status` is the command that exits 1 for that.
func (in invocation) status(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"json"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	// status answers "is everything fine?", so a broken project file or a
	// daemon that does not answer is part of the answer, not a failure.
	proj, _, projErr := in.currentProject()
	snap, daemonErr := in.gather(ctx, 5, false)
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	if flags["json"] != "" {
		doc := statusDoc{Version: Version, Project: proj, Daemon: snap.Status, Pipelines: countFlows(snap.Pipelines),
			RecentRuns: snap.Runs, Home: string(h)}
		if projErr != nil {
			doc.ProjectError = firstLine(projErr.Error())
		}
		if daemonErr != nil {
			doc.DaemonError = firstLine(daemonErr.Error())
		}
		if doc.RecentRuns == nil {
			doc.RecentRuns = []api.Run{}
		}
		if snap.Status != nil {
			url := "http://" + snap.Status.Address + "/"
			doc.DesktopURL = &url
		}
		return writeJSON(in.stdout, doc)
	}
	line := func(label, text string) { fmt.Fprintf(in.stdout, "%-9s  %s\n", label, clean(text)) }
	line("TRUSS", Version+" ("+runtime.GOOS+"/"+runtime.GOARCH+")")
	switch {
	case projErr != nil:
		line("Project", "unreadable: "+firstLine(projErr.Error()))
	case proj == nil:
		line("Project", "none here (no truss.yaml in "+workingRoot(in.cwd)+")")
	default:
		line("Project", fmt.Sprintf("%s, %d %s in .truss/flows/", proj.Name, proj.Flows, plural(proj.Flows, "flow", "flows")))
	}
	if daemonErr != nil {
		line("Daemon", "not answering: "+firstLine(daemonErr.Error()))
		line("Home", string(h))
		return 0
	}
	if snap.Status == nil {
		line("Daemon", "not running (start it with: truss daemon start)")
		line("Home", string(h))
		return 0
	}
	st := snap.Status
	line("Daemon", fmt.Sprintf("running, pid %d, up %s, %s", st.PID, uptime(snap), st.Address))
	line("Desktop", "http://"+st.Address+"/ (open it with: truss desktop)")
	line("Runs", fmt.Sprintf("%d running, %d queued (limit %d at once)", st.RunningRuns, st.QueuedRuns, st.MaxParallelRuns))
	n := countFlows(snap.Pipelines)
	line("Flows", fmt.Sprintf("%d registered: %d ready, %d running, %d paused, %d invalid", n.Registered, n.Ready, n.Running, n.Paused, n.Invalid))
	if len(snap.Runs) == 0 {
		line("Last run", "none yet")
	} else {
		r := snap.Runs[0]
		line("Last run", fmt.Sprintf("%s %s %s (truss runs show %s)", r.Pipeline, r.Status, age(snap.Now, r.CreatedAt), r.ID))
	}
	line("Home", string(h))
	return 0
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// dashboard draws the ASCII dashboard: daemon, load, flows and runs in
// frames. On a terminal it draws again every two seconds until interrupted;
// with --once, or when output is a pipe or file, it draws once.
func (in invocation) dashboard(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"once"})
	if err != nil || len(pos) != 0 {
		return in.usage()
	}
	live := flags["once"] == "" && canRedraw(in.stdout)
	width := screenWidth(in.env)
	for {
		snap, err := in.gather(ctx, 8, true)
		if err != nil {
			if live && ctx.Err() != nil {
				return 0
			}
			return in.fail(err)
		}
		_, flows, _ := in.currentProject()
		frame := renderDashboard(snap, flows, width)
		if !live {
			fmt.Fprint(in.stdout, frame)
			return 0
		}
		// Home, then clear: a full redraw with no flicker of scrolling.
		fmt.Fprint(in.stdout, "\x1b[H\x1b[2J"+frame+"\nRefreshes every "+refreshEvery.String()+". Ctrl-C quits.\n")
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(refreshEvery):
		}
	}
}

// canRedraw reports whether w is a terminal that takes cursor commands. The
// Windows console does not until it is switched to virtual terminal mode,
// which TRUSS does not do, so there the dashboard draws once.
func canRedraw(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && runtime.GOOS != "windows" && isTerminal(f)
}

// renderDashboard draws one frame. project flows are only shown when no
// daemon runs, so that the screen still says what could be registered.
func renderDashboard(snap snapshot, projectFlows []project.Flow, width int) string {
	var b strings.Builder
	stamp := snap.Now.Local().Format("2006-01-02 15:04:05")
	if snap.Status == nil {
		lines := []string{
			"Daemon   not running",
			"         start it with: truss daemon start",
		}
		b.WriteString(box("TRUSS "+Version+"  "+stamp, lines, width))
		if len(projectFlows) > 0 {
			var l []string
			for _, f := range projectFlows {
				l = append(l, f.Name)
			}
			b.WriteString(box(fmt.Sprintf("Flows in this project (%d)", len(l)), l, width))
		}
		return b.String()
	}
	st := snap.Status
	meter := max(width-4-52, 8)
	lines := []string{
		fmt.Sprintf("Daemon   running  pid %d  %s  up %s", st.PID, st.Address, uptime(snap)),
		fmt.Sprintf("Runs     %d running, %d queued   %s %d of %d slots", st.RunningRuns, st.QueuedRuns,
			bar(float64(st.RunningRuns)/float64(max(st.MaxParallelRuns, 1)), min(meter, 20)), st.RunningRuns, st.MaxParallelRuns),
	}
	if m := snap.Metrics; m != nil && m.Host != nil {
		mem := 0.0
		if m.Host.MemoryTotalBytes > 0 {
			mem = float64(m.Host.MemoryUsedBytes) / float64(m.Host.MemoryTotalBytes)
		}
		lines = append(lines, fmt.Sprintf("Host     CPU %s %3.0f%%   Mem %s %3.0f%% (%s of %s)",
			bar(m.Host.CPUPercent/100, 10), m.Host.CPUPercent, bar(mem, 10), mem*100,
			bytesText(m.Host.MemoryUsedBytes), bytesText(m.Host.MemoryTotalBytes)))
	}
	b.WriteString(box("TRUSS "+Version+"  "+stamp, lines, width))

	if len(snap.Pipelines) == 0 {
		b.WriteString(box("Flows (0)", []string{"none registered (add one with: truss register <name>)"}, width))
	} else {
		rows := [][]string{{"STATE", "FLOW", "NEXT RUN", "LAST RUN"}}
		for i, p := range snap.Pipelines {
			if i == 8 {
				break
			}
			rows = append(rows, []string{pipelineState(p), p.Name, nextRun(p), lastRunAge(p, snap.Now)})
		}
		l := columns(rows, width-4)
		if more := len(snap.Pipelines) - 8; more > 0 {
			l = append(l, fmt.Sprintf("... and %d more (truss pipelines)", more))
		}
		b.WriteString(box(fmt.Sprintf("Flows (%d)", len(snap.Pipelines)), l, width))
	}

	if len(snap.Runs) == 0 {
		b.WriteString(box("Runs", []string{"no runs yet"}, width))
	} else {
		rows := [][]string{{"RUN", "FLOW", "STATUS", "WHEN", "TOOK"}}
		for _, r := range snap.Runs {
			rows = append(rows, []string{r.ID, r.Pipeline, r.Status, age(snap.Now, r.CreatedAt), elapsed(r.StartedAt, r.FinishedAt, snap.Now)})
		}
		b.WriteString(box("Latest runs", columns(rows, width-4), width))
	}
	return b.String()
}

func lastRunAge(p api.Pipeline, now time.Time) string {
	if p.LastRun == nil {
		return "-"
	}
	return p.LastRun.Status + " " + age(now, p.LastRun.CreatedAt)
}

// env prints where TRUSS keeps its files and what it sees around it.
func (in invocation) environment(ctx context.Context) int {
	if len(in.args) != 0 {
		return in.usage()
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	source := "default"
	if explicitHome(in.env) {
		source = "from " + home.EnvVar
	}
	exists := func(path string) string {
		if _, err := os.Stat(path); err != nil {
			return "(none yet)"
		}
		return ""
	}
	line := func(label, text string) { fmt.Fprintf(in.stdout, "%-12s  %s\n", label, clean(strings.TrimSpace(text))) }
	line("TRUSS", Version)
	line("System", runtime.GOOS+"/"+runtime.GOARCH+", "+runtime.Version())
	line("Home", fmt.Sprintf("%s (%s)", h, source))
	line("Registry", h.Registry()+" "+exists(h.Registry()))
	line("Secrets", h.Secrets()+" "+exists(h.Secrets())+" (names: truss secret list)")
	line("Daemon log", h.DaemonLog()+" "+exists(h.DaemonLog()))
	root := workingRoot(in.cwd)
	if proj, err := project.Open(root); err == nil {
		line("Project", fmt.Sprintf("%s in %s", proj.Name, proj.Root))
		line("Flows", filepath.Join(root, ".truss", "flows"))
		line("State", filepath.Join(root, ".truss", "state"))
	} else if errors.Is(err, project.ErrNoProject) {
		line("Project", "none (no truss.yaml in "+root+")")
	} else {
		line("Project", "unreadable: "+err.Error())
	}
	switch c, err := in.dial(ctx); {
	case err == nil:
		line("Daemon", "running at "+c.Endpoint().Address)
	case errors.Is(err, api.ErrNotRunning):
		line("Daemon", "not running")
	default:
		line("Daemon", "unknown: "+err.Error())
	}
	return 0
}

// projectCmd prints the project of the working directory.
func (in invocation) projectCmd(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"json"})
	if err != nil || len(pos) > 1 || (len(pos) == 1 && pos[0] != "show") {
		return in.usage()
	}
	proj, flows, err := in.currentProject()
	if err != nil {
		return in.fail(err)
	}
	if proj == nil {
		return in.fail(fmt.Errorf("%w: there is no %s in %s (run TRUSS from the project folder)", project.ErrNoProject, project.FileName, workingRoot(in.cwd)))
	}
	snap, err := in.gather(ctx, 1, false)
	if err != nil {
		return in.fail(err)
	}
	registered := 0
	for _, f := range flows {
		if _, ok := findRegistered(snap.Pipelines, f.Path); ok {
			registered++
		}
	}
	if flags["json"] != "" {
		return writeJSON(in.stdout, struct {
			projectDoc
			Registered *int `json:"registered,omitempty"`
		}{*proj, daemonCount(snap, registered)})
	}
	line := func(label, text string) { fmt.Fprintf(in.stdout, "%-12s  %s\n", label, clean(text)) }
	line("Project", proj.Name)
	if proj.Description != "" {
		line("Description", proj.Description)
	}
	line("Root", proj.Root)
	line("Flows", fmt.Sprintf("%d (list them with: truss flows)", proj.Flows))
	if snap.Status == nil {
		line("Registered", "unknown (the daemon is not running)")
	} else {
		line("Registered", fmt.Sprintf("%d of %d", registered, proj.Flows))
	}
	return 0
}

func daemonCount(snap snapshot, n int) *int {
	if snap.Status == nil {
		return nil
	}
	return &n
}

// findRegistered finds the registration of a pipeline file.
func findRegistered(list []api.Pipeline, path string) (api.Pipeline, bool) {
	for _, p := range list {
		if samePath(p.Path, path) {
			return p, true
		}
	}
	return api.Pipeline{}, false
}

// samePath reports whether two spellings name one file: the same cleaned and
// link-resolved path, the same file on disk (which covers case-insensitive
// file systems and short names), or, when a file is gone, the same name up to
// case on Windows and macOS, whose default file systems ignore case.
func samePath(a, b string) bool {
	a, b = sameFileKey(a), sameFileKey(b)
	if a == b {
		return true
	}
	if ia, err := os.Stat(a); err == nil {
		if ib, err := os.Stat(b); err == nil {
			return os.SameFile(ia, ib)
		}
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return false
}

// sameFileKey makes two spellings of one path comparable.
func sameFileKey(path string) string {
	path = filepath.Clean(path)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path
}

// projectGroup is the registrations that come from one project folder.
type projectGroup struct {
	Name       string `json:"name"`
	Root       string `json:"root"`
	Registered int    `json:"registered"`
	Running    int    `json:"running"`
	Paused     int    `json:"paused"`
	Invalid    int    `json:"invalid"`
}

// projects lists the projects the daemon knows, grouped from the paths of its
// registrations. A file outside any project's flows folder is listed apart.
func (in invocation) projects(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"json"})
	if err != nil || len(pos) > 1 || (len(pos) == 1 && pos[0] != "list") {
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
	groups := map[string]*projectGroup{}
	for _, p := range list {
		root, name := projectOfFile(p.Path)
		key := root
		if root == "" {
			key, name = "", "(no project)"
		}
		g := groups[key]
		if g == nil {
			g = &projectGroup{Name: name, Root: root}
			groups[key] = g
		}
		g.Registered++
		switch pipelineState(p) {
		case "running":
			g.Running++
		case "paused":
			g.Paused++
		case "invalid":
			g.Invalid++
		}
	}
	out := make([]projectGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Root == "" || out[j].Root == "" {
			return out[j].Root == "" && out[i].Root != ""
		}
		return out[i].Name < out[j].Name || (out[i].Name == out[j].Name && out[i].Root < out[j].Root)
	})
	if flags["json"] != "" {
		return writeJSON(in.stdout, out)
	}
	if len(out) == 0 {
		fmt.Fprintln(in.stdout, "no projects: nothing is registered (add a flow with: truss register <name>)")
		return 0
	}
	rows := [][]string{{"PROJECT", "FLOWS", "RUNNING", "PAUSED", "INVALID", "ROOT"}}
	for _, g := range out {
		root := g.Root
		if root == "" {
			root = "-"
		}
		rows = append(rows, []string{g.Name, fmt.Sprint(g.Registered), fmt.Sprint(g.Running), fmt.Sprint(g.Paused), fmt.Sprint(g.Invalid), root})
	}
	printColumns(in.stdout, rows)
	return 0
}

// scheduleRow is one schedule of one registered pipeline.
type scheduleRow struct {
	Pipeline string     `json:"pipeline"`
	Cron     string     `json:"cron"`
	Timezone string     `json:"timezone,omitempty"`
	Missed   string     `json:"missed"`
	Next     *time.Time `json:"next,omitempty"`
	Paused   bool       `json:"paused"`
}

// schedules lists every schedule of every registered pipeline, soonest first.
func (in invocation) schedules(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"json"})
	if err != nil || len(pos) > 1 || (len(pos) == 1 && pos[0] != "list") {
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
	rows := []scheduleRow{}
	for _, p := range list {
		for _, s := range p.Schedules {
			rows = append(rows, scheduleRow{Pipeline: p.Name, Cron: s.Cron, Timezone: s.Timezone, Missed: s.Missed, Next: s.Next, Paused: p.Paused})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i].Next, rows[j].Next
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return a.Before(*b)
	})
	if flags["json"] != "" {
		return writeJSON(in.stdout, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(in.stdout, "no schedules: no registered pipeline has an on.schedule entry")
		return 0
	}
	out := [][]string{{"NEXT RUN", "PIPELINE", "CRON", "ZONE", "MISSED", "STATE"}}
	for _, r := range rows {
		zone, state := r.Timezone, "active"
		if zone == "" {
			zone = "local"
		}
		if r.Paused {
			state = "paused"
		}
		out = append(out, []string{local(r.Next), r.Pipeline, r.Cron, zone, r.Missed, state})
	}
	printColumns(in.stdout, out)
	return 0
}
