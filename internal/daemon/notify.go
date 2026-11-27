package daemon

import (
	"context"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/notify"
	"github.com/afri-bit/truss/internal/secrets"
)

// notifyRun sends the notifications a flow's notify block asks for about a
// run that ended (spec 030). It returns at once; sending happens on its own
// goroutine and never changes the run.
func (d *Daemon) notifyRun(run api.Run) {
	d.mu.Lock()
	def, ok := d.defs[run.Pipeline]
	d.mu.Unlock()
	if !ok || def.err != nil || def.def.Notify == nil {
		return
	}
	e, ok := d.registry.Get(run.Pipeline)
	if !ok {
		return
	}
	o := notify.Outcome{Flow: run.Pipeline, RunID: run.ID, Status: run.Status, Reason: run.Reason,
		Trigger: run.Trigger, Missed: run.Status == api.RunSkipped && run.Reason == missedReason}
	if run.FinishedAt != nil {
		o.Ended = *run.FinishedAt
	}
	for _, r := range d.runs.List(run.Pipeline, 0) {
		if r.ID != run.ID && !r.Active() && r.CreatedAt.Before(run.CreatedAt) && r.Status != api.RunSkipped {
			o.Previous = r.Status
			break
		}
	}
	n := def.def.Notify
	if !notify.Wanted(n, o) {
		return
	}
	store := secrets.Open(d.opts.Home.Secrets())
	sender := notify.Sender{
		Secret: func(name string) (string, bool) {
			all, err := store.Load()
			if err != nil {
				return "", false
			}
			v, ok := all[name]
			return v, ok
		},
		Shell: executor.ShellCommand,
		Dir:   workingDir(e.Path),
		Env:   d.opts.Env,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*notify.Timeout+5*time.Second)
		defer cancel()
		for _, err := range sender.Send(ctx, n, o) {
			d.logf("flow %s: notification of run %s not sent: %v", run.Pipeline, run.ID, err)
		}
	}()
}
