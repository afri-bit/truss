// Package daemon is the long-running TRUSS service.
//
// It ties the other packages together: the registry says which pipelines
// exist, the scheduler says when their schedules are due, the run manager
// executes and remembers runs, and an HTTP API on the loopback interface
// lets the CLI and TRUSS Desktop see and steer all of it.
package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/registry"
	"github.com/afri-bit/truss/internal/runs"
	"github.com/afri-bit/truss/internal/scheduler"
	"github.com/afri-bit/truss/internal/secrets"
)

// DefaultAddress is where the daemon listens unless told otherwise.
const DefaultAddress = "127.0.0.1:7878"

// missedReason explains a skipped schedule in the run history.
const missedReason = "missed: the scheduled time passed while TRUSS was not watching the clock (machine off or asleep, or daemon stopped)"

// Options configures a Daemon.
type Options struct {
	Home home.Dir
	// Address is a loopback host:port. Port 0 picks a free port.
	Address string
	// MaxParallelRuns limits runs that execute at once; 0 means the default.
	MaxParallelRuns int
	// Version is the product version reported by the API.
	Version string
	// Env is the environment steps start from ("KEY=value" entries).
	Env []string
	// Log receives one line per notable event. Nil discards them.
	Log io.Writer
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// Tick is how often the schedules and definition files are checked;
	// 0 means once a second.
	Tick time.Duration
	// Grace is how late a schedule may fire; 0 means scheduler.DefaultGrace.
	Grace time.Duration
}

// definition is the daemon's current reading of one registered file.
type definition struct {
	modTime time.Time
	size    int64
	def     pipeline.Pipeline
	plan    pipeline.ExecutionPlan
	err     error
	// scheduled records whether the schedules are active (valid and not paused).
	scheduled bool
}

// Daemon is one running TRUSS service.
type Daemon struct {
	opts     Options
	registry *registry.Registry
	runs     *runs.Manager
	listener net.Listener
	endpoint api.Endpoint

	mu    sync.Mutex // guards sched and defs
	sched *scheduler.Scheduler
	defs  map[string]*definition

	stopOnce sync.Once
	stop     chan struct{}
}

type clockFile struct {
	// LastSeen is the last moment the scheduler looked at the time.
	LastSeen time.Time `json:"last_seen"`
}

// Start opens the state in the home directory, starts listening and writes
// the endpoint file. It fails when another daemon already serves this home.
// Call Serve next.
func Start(ctx context.Context, opts Options) (*Daemon, error) {
	if opts.Address == "" {
		opts.Address = DefaultAddress
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Tick <= 0 {
		opts.Tick = time.Second
	}
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	if err := checkLoopback(opts.Address); err != nil {
		return nil, err
	}
	if err := opts.Home.Ensure(); err != nil {
		return nil, err
	}
	if c, err := api.Dial(ctx, opts.Home); err == nil {
		return nil, fmt.Errorf("a daemon is already running for %s (pid %d, %s)", opts.Home, c.Endpoint().PID, c.Endpoint().Address)
	}

	reg, err := registry.Open(opts.Home.Registry())
	if err != nil {
		return nil, err
	}
	mgr, err := runs.Open(runs.Options{Dir: opts.Home.Runs(), MaxParallel: opts.MaxParallelRuns, Env: opts.Env, Secrets: secrets.Open(opts.Home.Secrets()).Load, Now: opts.Now})
	if err != nil {
		return nil, err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", opts.Address)
	if err != nil {
		mgr.Close()
		return nil, fmt.Errorf("listen on %s: %w", opts.Address, err)
	}
	token := make([]byte, 24)
	if _, err := rand.Read(token); err != nil {
		_ = ln.Close()
		mgr.Close()
		return nil, fmt.Errorf("create access token: %w", err)
	}
	now := opts.Now()
	d := &Daemon{
		opts:     opts,
		registry: reg,
		runs:     mgr,
		listener: ln,
		endpoint: api.Endpoint{Address: ln.Addr().String(), Token: hex.EncodeToString(token), PID: os.Getpid(), StartedAt: now, Version: opts.Version},
		sched:    scheduler.New(opts.Grace),
		defs:     map[string]*definition{},
		stop:     make(chan struct{}),
	}

	// Count schedules from the last moment a daemon watched the clock, so the
	// first tick can tell which times were missed in between.
	since := now
	var clock clockFile
	if home.ReadJSON(opts.Home.Clock(), &clock) == nil && !clock.LastSeen.IsZero() && clock.LastSeen.Before(now) {
		since = clock.LastSeen
	}
	d.refresh(since)

	if err := home.WriteJSON(opts.Home.Endpoint(), d.endpoint); err != nil {
		_ = ln.Close()
		mgr.Close()
		return nil, err
	}
	return d, nil
}

// Endpoint returns where the daemon listens and the token clients need.
func (d *Daemon) Endpoint() api.Endpoint { return d.endpoint }

// Stop asks Serve to return. It is safe to call more than once.
func (d *Daemon) Stop() { d.stopOnce.Do(func() { close(d.stop) }) }

// Serve answers API requests and fires schedules until ctx is done or Stop
// is called. It then cancels active runs, removes the endpoint file and
// returns.
func (d *Daemon) Serve(ctx context.Context) error {
	server := &http.Server{Handler: d.handler(), ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(d.listener) }()
	d.logf("daemon started on %s (pid %d, home %s)", d.endpoint.Address, d.endpoint.PID, d.opts.Home)

	ticker := time.NewTicker(d.opts.Tick)
	defer ticker.Stop()
	var err error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-d.stop:
			break loop
		case err = <-served:
			break loop
		case <-ticker.C:
			d.tick()
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	d.runs.Close()
	d.saveClock(d.opts.Now())
	_ = os.Remove(d.opts.Home.Endpoint())
	d.logf("daemon stopped")
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve API: %w", err)
	}
	return nil
}

// tick re-reads changed definition files and acts on due schedules.
func (d *Daemon) tick() {
	now := d.opts.Now()
	d.refresh(now)
	d.mu.Lock()
	decisions := d.sched.Tick(now)
	d.mu.Unlock()

	for _, dec := range decisions {
		switch dec.Action {
		case scheduler.Missed:
			if _, err := d.runs.Record(dec.Pipeline, api.TriggerSchedule, dec.ScheduledFor, api.RunSkipped, missedReason); err != nil {
				d.logf("pipeline %s: record missed schedule: %v", dec.Pipeline, err)
			}
			d.logf("pipeline %s: skipped the schedule %q for %s (missed)", dec.Pipeline, dec.Cron, stamp(dec.ScheduledFor))
		case scheduler.Fire, scheduler.CatchUp:
			trigger := api.TriggerSchedule
			if dec.Action == scheduler.CatchUp {
				trigger = api.TriggerCatchUp
			}
			run, err := d.submit(dec.Pipeline, trigger, dec.ScheduledFor)
			if err != nil {
				d.logf("pipeline %s: schedule %q could not start a run: %v", dec.Pipeline, dec.Cron, err)
				continue
			}
			d.logf("pipeline %s: schedule %q for %s -> run %s (%s)", dec.Pipeline, dec.Cron, stamp(dec.ScheduledFor), run.ID, run.Status)
		}
	}
	// Remember the moment right away after acting, so a crash cannot make
	// the same time fire twice; otherwise twice a minute is enough.
	if len(decisions) > 0 || now.Second()%30 == 0 {
		d.saveClock(now)
	}
}

func (d *Daemon) saveClock(now time.Time) {
	if err := home.WriteJSON(d.opts.Home.Clock(), clockFile{LastSeen: now}); err != nil {
		d.logf("save clock: %v", err)
	}
}

// refresh brings defs and the scheduler in line with the registry and the
// files on disk. Schedules of new or changed pipelines count from since.
func (d *Daemon) refresh(since time.Time) {
	entries := d.registry.List()
	d.mu.Lock()
	defer d.mu.Unlock()
	known := make(map[string]bool, len(entries))
	for _, e := range entries {
		known[e.Name] = true
		d.load(e, since, false)
	}
	for name := range d.defs {
		if !known[name] {
			delete(d.defs, name)
			d.sched.Remove(name)
		}
	}
}

// load reads one registered file when it changed (or force is set) and
// updates its schedules. The caller holds d.mu.
func (d *Daemon) load(e registry.Entry, since time.Time, force bool) *definition {
	old := d.defs[e.Name]
	info, statErr := os.Stat(e.Path)
	// A file is read again only when it changed; a missing file only when it appears.
	unchanged := old != nil && statErr == nil && !old.modTime.IsZero() && info.ModTime().Equal(old.modTime) && info.Size() == old.size
	missingStill := old != nil && statErr != nil && old.modTime.IsZero()
	cur := old
	if force || (!unchanged && !missingStill) {
		cur = &definition{}
		if statErr != nil {
			cur.err = fmt.Errorf("read pipeline definition: %w", statErr)
		} else {
			cur.modTime, cur.size = info.ModTime(), info.Size()
			cur.def, cur.plan, cur.err = readDefinition(e.Path)
		}
		d.defs[e.Name] = cur
	}
	want := cur.err == nil && !e.Paused && len(cur.def.Schedules) > 0
	if cur != old || want != cur.scheduled {
		if want {
			d.sched.Set(e.Name, cur.def.Schedules, since)
		} else {
			d.sched.Remove(e.Name)
		}
		cur.scheduled = want
	}
	return cur
}

func readDefinition(path string) (pipeline.Pipeline, pipeline.ExecutionPlan, error) {
	def, err := pipeline.ParseFile(context.Background(), path)
	if err != nil {
		return def, pipeline.ExecutionPlan{}, err
	}
	if err := pipeline.Validate(def); err != nil {
		return def, pipeline.ExecutionPlan{}, err
	}
	plan, err := pipeline.Plan(def)
	return def, plan, err
}

// submit reads the definition as it is now and requests a run of it.
func (d *Daemon) submit(name, trigger string, scheduledFor time.Time) (api.Run, error) {
	e, ok := d.registry.Get(name)
	if !ok {
		return api.Run{}, fmt.Errorf("%w: %q", registry.ErrNotFound, name)
	}
	d.mu.Lock()
	cur := d.load(e, d.opts.Now(), true)
	d.mu.Unlock()
	if cur.err != nil {
		if trigger != api.TriggerManual {
			_, _ = d.runs.Record(name, trigger, scheduledFor, api.RunFailed, cur.err.Error())
		}
		return api.Run{}, &invalidError{cur.err}
	}
	return d.runs.Submit(runs.Request{
		Pipeline:     name,
		Definition:   cur.def,
		Dir:          filepath.Dir(e.Path),
		Trigger:      trigger,
		ScheduledFor: scheduledFor,
	})
}

// invalidError marks a definition that cannot run as it is.
type invalidError struct{ err error }

func (e *invalidError) Error() string { return e.err.Error() }
func (e *invalidError) Unwrap() error { return e.err }

// view builds the API form of one registered pipeline.
func (d *Daemon) view(e registry.Entry) api.Pipeline {
	p := api.Pipeline{Name: e.Name, Path: e.Path, Paused: e.Paused, RegisteredAt: e.RegisteredAt, Schedules: []api.Schedule{}, Steps: []api.PlanStep{}}
	d.mu.Lock()
	cur := d.defs[e.Name]
	if cur == nil {
		cur = d.load(e, d.opts.Now(), false)
	}
	next := d.sched.Next(e.Name)
	d.mu.Unlock()

	p.ActiveRuns, p.LastRun = d.runs.Summary(e.Name)
	if cur.err != nil {
		p.Error = cur.err.Error()
		return p
	}
	p.Valid = true
	p.DefinitionName = cur.def.Name
	p.Concurrency = cur.plan.Concurrency
	p.Overlap = string(cur.def.Overlap)
	if p.Overlap == "" {
		p.Overlap = string(pipeline.OverlapQueue)
	}
	for i, s := range cur.def.Schedules {
		sc := api.Schedule{Cron: s.Cron, Timezone: s.Timezone, Missed: string(s.Missed)}
		if i < len(next) && !next[i].IsZero() {
			t := next[i]
			sc.Next = &t
		}
		p.Schedules = append(p.Schedules, sc)
	}
	for _, s := range cur.plan.Graph {
		p.Steps = append(p.Steps, api.PlanStep{ID: s.ID, Needs: s.Needs})
	}
	return p
}

func (d *Daemon) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(d.opts.Log, "%s %s\n", stamp(d.opts.Now()), fmt.Sprintf(format, args...))
}

func stamp(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

// checkLoopback refuses addresses that would expose the API to the network.
func checkLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("daemon address %q: %w", address, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("daemon address %q is not a loopback address; the daemon only serves this machine", address)
	}
	return nil
}
