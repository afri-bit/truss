package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
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

func (in invocation) register(ctx context.Context) int {
	pos, flags, err := options(in.args, []string{"name"}, nil)
	if err != nil || len(pos) > 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	p, err := c.Register(ctx, api.RegisterRequest{Path: definitionPath(in.cwd, pos), Name: flags["name"]})
	if err != nil {
		return in.fail(err)
	}
	fmt.Fprintf(in.stdout, "registered %s (%s)\n", p.Name, p.Path)
	for _, s := range p.Schedules {
		if s.Next != nil {
			fmt.Fprintf(in.stdout, "schedule %s: next run %s\n", s.Cron, local(s.Next))
		}
	}
	return 0
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

func (in invocation) pipelines(ctx context.Context) int {
	if len(in.args) != 0 {
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

func (in invocation) runs(ctx context.Context) int {
	if len(in.args) > 1 {
		return in.usage()
	}
	c, err := in.dial(ctx)
	if err != nil {
		return in.fail(err)
	}
	name := strings.Join(in.args, "")
	list, err := c.Runs(ctx, name, 30)
	if err != nil {
		return in.fail(err)
	}
	if len(list) == 0 {
		fmt.Fprintln(in.stdout, "no runs yet")
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
