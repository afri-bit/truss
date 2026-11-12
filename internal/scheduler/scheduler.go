// Package scheduler decides when a schedule is due.
//
// It owns no timer and starts nothing. The daemon calls Tick with the current
// time and receives decisions; that keeps the rules below easy to test.
//
// A schedule fires when the clock reaches its time while someone is watching.
// If the time has already passed by more than the grace period (the machine
// was off or asleep, or the daemon was not running) the time was missed. A
// missed time is skipped by default: a job planned for 09:00 does not start
// at 11:00 because the laptop was opened late. A schedule can ask for one
// catch-up run instead with `missed: run`.
package scheduler

import (
	"sort"
	"time"

	"github.com/afri-bit/truss/internal/cron"
	"github.com/afri-bit/truss/internal/pipeline"
)

// DefaultGrace is how late a schedule may fire and still count as on time.
// It absorbs timer drift and a busy machine, not a closed laptop.
const DefaultGrace = 2 * time.Minute

// Action is what the scheduler decided about one scheduled time.
type Action string

const (
	// Fire means the time was reached on time: start a run.
	Fire Action = "fire"
	// CatchUp means times were missed and the schedule asks for one run.
	CatchUp Action = "catch-up"
	// Missed means times were missed and are skipped.
	Missed Action = "missed"
)

// Decision is one outcome of a Tick.
type Decision struct {
	Pipeline string
	Action   Action
	// ScheduledFor is the time the schedule named. For missed times it is the
	// first one that was missed.
	ScheduledFor time.Time
	// Cron is the expression that produced the decision, as written.
	Cron string
}

type entry struct {
	expr   cron.Expr
	text   string
	loc    *time.Location
	missed pipeline.Missed
	next   time.Time
}

// Scheduler tracks the next time of every schedule of every pipeline.
// It is not safe for concurrent use; the daemon calls it from one goroutine.
type Scheduler struct {
	grace   time.Duration
	entries map[string][]entry
}

// New returns an empty scheduler. A grace of zero means DefaultGrace.
func New(grace time.Duration) *Scheduler {
	if grace <= 0 {
		grace = DefaultGrace
	}
	return &Scheduler{grace: grace, entries: map[string][]entry{}}
}

// Set replaces the schedules of a pipeline. Times are counted from since:
// pass the current time for a new or changed pipeline, or the moment the
// clock was last watched to let Tick notice what was missed in between.
// Schedules that do not parse are ignored; validation reports them earlier.
func (s *Scheduler) Set(name string, schedules []pipeline.Schedule, since time.Time) {
	list := make([]entry, 0, len(schedules))
	for _, sc := range schedules {
		expr, err := cron.Parse(sc.Cron)
		if err != nil {
			continue
		}
		loc := time.Local
		if sc.Timezone != "" {
			if l, err := time.LoadLocation(sc.Timezone); err == nil {
				loc = l
			}
		}
		list = append(list, entry{expr: expr, text: sc.Cron, loc: loc, missed: sc.Missed, next: expr.Next(since.In(loc))})
	}
	if len(list) == 0 {
		delete(s.entries, name)
		return
	}
	s.entries[name] = list
}

// Remove forgets the schedules of a pipeline.
func (s *Scheduler) Remove(name string) { delete(s.entries, name) }

// Next returns the next time of each schedule of a pipeline, in the order
// the schedules were given to Set.
func (s *Scheduler) Next(name string) []time.Time {
	var times []time.Time
	for _, e := range s.entries[name] {
		times = append(times, e.next)
	}
	return times
}

// Tick reports every schedule whose time has come by now and moves each one
// to its next time after now. Decisions are ordered by pipeline name.
func (s *Scheduler) Tick(now time.Time) []Decision {
	var out []Decision
	for name, list := range s.entries {
		for i := range list {
			e := &list[i]
			if e.next.IsZero() || now.Before(e.next) {
				continue
			}
			d := Decision{Pipeline: name, ScheduledFor: e.next, Cron: e.text}
			switch {
			case now.Sub(e.next) <= s.grace:
				d.Action = Fire
			case e.missed == pipeline.MissedRun:
				d.Action = CatchUp
			default:
				d.Action = Missed
			}
			out = append(out, d)
			e.next = e.expr.Next(now.In(e.loc))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pipeline < out[j].Pipeline })
	return out
}
