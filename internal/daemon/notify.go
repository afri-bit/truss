package daemon

import (
	"strings"
	"unicode"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/notify"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/secrets"
)

// notifyRun sends the notifications a flow's notify block asks for about a
// run that ended (spec 030). It returns at once; sending happens on its own
// goroutine and never changes the run.
func (d *Daemon) notifyRun(run api.Run) {
	// A run tracked from a shell (truss run --track) is that shell's to
	// notify about, with --notify; the daemon only records it.
	if run.Trigger == api.TriggerLocal {
		return
	}
	d.mu.Lock()
	def, ok := d.defs[run.Pipeline]
	var n *pipeline.Notify
	if ok {
		n = def.notify // the last valid block: a file that stopped loading still notifies
	}
	d.mu.Unlock()
	if n == nil {
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
		Dir: workingDir(e.Path),
		Env: d.opts.Env,
	}
	d.notifying.Add(1)
	go func() {
		defer d.notifying.Done()
		for _, err := range sender.Send(d.notifyCtx, n, o) {
			d.logf("flow %s: notification of run %s not sent: %s", run.Pipeline, run.ID, strings.Map(noControl, err.Error()))
		}
	}()
}

// noControl drops control characters from text for the daemon's log.
func noControl(r rune) rune {
	if unicode.IsControl(r) {
		return -1
	}
	return r
}
