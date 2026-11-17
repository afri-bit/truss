package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/project"
)

// followEvery is how often a followed run log is read again.
const followEvery = 250 * time.Millisecond

// dial connects to the daemon of this invocation's TRUSS home.
func (in invocation) dial(ctx context.Context) (*api.Client, error) {
	h, err := home.Resolve(in.env)
	if err != nil {
		return nil, err
	}
	return api.Dial(ctx, h)
}

// register makes pipelines known to the daemon: one by name or path, or every
// flow of the project with --all (spec 016 FR-012).
func (in invocation) register(ctx context.Context) int {
	pos, flags, err := options(in.args, []string{"name"}, []string{"all"})
	if err != nil || len(pos) > 1 {
		return in.usage()
	}
	all := flags["all"] != ""
	_, named := flags["name"]
	if all && (named || len(pos) > 0) {
		if named {
			return in.fail(errors.New("--name cannot be used with --all"))
		}
		return in.usage()
	}
	var files []pipelineFile
	switch {
	case all:
		proj, err := project.Open(workingRoot(in.cwd))
		if err != nil {
			return in.fail(err)
		}
		flows, err := proj.Flows()
		if err != nil {
			return in.fail(err)
		}
		if len(flows) == 0 {
			return in.fail(errors.New("nothing to register; " + project.AvailableText(nil)))
		}
		for _, f := range flows {
			files = append(files, pipelineFile{path: f.Path, display: ".truss/flows/" + f.File, stem: f.Name})
		}
	case len(pos) == 0:
		return in.fail(needFlowError(in.cwd, "register"))
	default:
		file, err := resolveFile(in.cwd, pos[0])
		if err != nil {
			return in.fail(err)
		}
		files = append(files, file)
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	code, done, scheduled := 0, 0, 0
	for _, file := range files {
		p, err := c.Register(ctx, api.RegisterRequest{Path: file.path, Name: flags["name"]})
		if err != nil {
			msg := err.Error()
			var status *api.StatusError
			if errors.As(err, &status) && status.Code == http.StatusConflict {
				if all {
					msg += "; register it alone with `truss register <flow> --name <other>`"
				} else {
					msg += "; register it under another name with --name"
				}
			}
			if all {
				msg = file.display + ": " + msg
			}
			fmt.Fprintf(in.stderr, "error: %s\n", msg)
			code = 1
			continue
		}
		fmt.Fprintf(in.stdout, "registered %s (%s)\n", p.Name, p.Path)
		done++
		if len(p.Schedules) > 0 {
			scheduled++
		}
		for _, s := range p.Schedules {
			if s.Next != nil {
				fmt.Fprintf(in.stdout, "schedule %s: next run %s\n", s.Cron, local(s.Next))
			}
		}
	}
	if all && code == 0 {
		noun := "pipelines"
		if done == 1 {
			noun = "pipeline"
		}
		fmt.Fprintf(in.stdout, "registered %d %s (%d with schedules)\n", done, noun, scheduled)
	}
	return code
}

func (in invocation) unregister(ctx context.Context) int {
	if len(in.args) != 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	if err := c.Unregister(ctx, in.args[0]); err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "unregistered %s\n", in.args[0])
	return 0
}

func pipelineState(p api.Pipeline) string {
	switch {
	case !p.Valid:
		return "invalid"
	case p.ActiveRuns > 0:
		return "running"
	case p.Paused:
		return "paused"
	}
	return "ready"
}

func nextRun(p api.Pipeline) string {
	var next *time.Time
	for _, s := range p.Schedules {
		if s.Next != nil && (next == nil || s.Next.Before(*next)) {
			next = s.Next
		}
	}
	if next == nil {
		return "-"
	}
	return local(next)
}

func lastRun(p api.Pipeline) string {
	if p.LastRun == nil {
		return "-"
	}
	return p.LastRun.Status + " " + local(&p.LastRun.CreatedAt)
}

func local(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func (in invocation) setPaused(ctx context.Context, paused bool) int {
	if len(in.args) != 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	p, err := c.SetPaused(ctx, in.args[0], paused)
	if err != nil {
		return in.fail(err)
	}
	if paused {
		fmt.Fprintf(in.stdout, "paused %s: its schedules will not start runs\n", p.Name)
	} else {
		fmt.Fprintf(in.stdout, "resumed %s: next run %s\n", p.Name, nextRun(p))
	}
	return 0
}

func (in invocation) trigger(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"follow", "f"})
	if err != nil || len(pos) != 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	run, err := c.Trigger(ctx, pos[0])
	if err != nil {
		return in.fail(err)
	}
	if run.Status == api.RunSkipped {
		fmt.Fprintf(in.stdout, "run %s skipped: %s\n", run.ID, run.Reason)
		return 0
	}
	if flags["follow"] == "" && flags["f"] == "" {
		fmt.Fprintf(in.stdout, "run %s %s (follow it with: truss logs %s --follow)\n", run.ID, run.Status, run.ID)
		return 0
	}
	return in.follow(ctx, c, run.ID)
}

func duration(r api.Run) string {
	if r.StartedAt == nil {
		return "-"
	}
	end := time.Now()
	if r.FinishedAt != nil {
		end = *r.FinishedAt
	}
	return end.Sub(*r.StartedAt).Round(100 * time.Millisecond).String()
}

func (in invocation) logs(ctx context.Context) int {
	pos, flags, err := options(in.args, nil, []string{"follow", "f"})
	if err != nil || len(pos) != 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	if flags["follow"] != "" || flags["f"] != "" {
		return in.follow(ctx, c, pos[0])
	}
	var offset int64
	for {
		data, next, _, err := c.Log(ctx, pos[0], offset)
		if err != nil {
			return in.fail(err)
		}
		if len(data) == 0 {
			return 0
		}
		_, _ = in.stdout.Write(data)
		offset = next
	}
}

// follow prints a run log as it grows and returns 0 only when the run
// succeeded. Interrupting it stops the printing, not the run.
func (in invocation) follow(ctx context.Context, c *api.Client, id string) int {
	var offset int64
	for {
		data, next, status, err := c.Log(ctx, id, offset)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return 1
			}
			return in.fail(err)
		}
		_, _ = in.stdout.Write(data)
		offset = next
		if len(data) > 0 {
			continue
		}
		switch status {
		case api.RunQueued, api.RunRunning:
			select {
			case <-ctx.Done():
				return 1
			case <-time.After(followEvery):
			}
		case api.RunSucceeded:
			return 0
		default:
			run, err := c.Run(ctx, id)
			if err == nil && run.Reason != "" {
				fmt.Fprintf(in.stderr, "run %s %s: %s\n", id, run.Status, run.Reason)
			}
			return 1
		}
	}
}

func (in invocation) cancel(ctx context.Context) int {
	if len(in.args) != 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	run, err := c.Cancel(ctx, in.args[0])
	if err != nil {
		return in.fail(err)
	}
	if run.Active() {
		fmt.Fprintf(in.stdout, "cancelling run %s\n", run.ID)
	} else {
		fmt.Fprintf(in.stdout, "run %s is %s\n", run.ID, run.Status)
	}
	return 0
}
