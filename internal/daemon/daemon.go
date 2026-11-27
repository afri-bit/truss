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
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/metrics"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/project"
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
	// KeepRuns is how many finished runs stay in the history; 0 means the
	// default.
	KeepRuns int
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
	// retry is set when the file exists but could not be read: it is read again
	// at every check, so a read that failed for a passing reason heals without
	// another edit.
	retry bool
	// named is pipeline.name from the last reading that got that far. It
	// survives a file that stops loading, so the name shown does not jump back.
	named string
	// checked is when the file was last looked at, by the clock of this machine.
	checked time.Time
	// armed holds the schedules given to the scheduler: those of the last
	// definition that was valid. A file that turns invalid keeps them, so a
	// fire time that passes is recorded as a failed run (FR-021).
	armed []pipeline.Schedule
	// scheduled records whether armed is registered with the scheduler.
	scheduled bool
}

// Daemon is one running TRUSS service.
type Daemon struct {
	// lastFresh is when a lookup last looked at every file without the throttle;
	// the caller holds mu.
	lastFresh time.Time
	opts      Options
	registry  *registry.Registry
	runs      *runs.Manager
	metrics   *metrics.Sampler
	listener  net.Listener
	endpoint  api.Endpoint

	mu    sync.Mutex // guards sched and defs
	sched *scheduler.Scheduler
	defs  map[string]*definition

	// watches is the on.file state per flow, and pendingRuns the on.run
	// runs waiting for a hook run of their flow to end (name to chain); only
	// the Serve goroutine uses them.
	watches     map[string]*fileWatch
	pendingRuns map[string]int
	// finished receives runs that ended, for on.run (spec 029); dropped
	// counts those that did not fit.
	finished chan finishedRun
	dropped  *atomic.Int64

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
	finished := make(chan finishedRun, 256)
	dropped := &atomic.Int64{}
	mgr, err := runs.Open(runs.Options{Dir: opts.Home.Runs(), MaxParallel: opts.MaxParallelRuns, Keep: opts.KeepRuns, Env: opts.Env, Secrets: secrets.Open(opts.Home.Secrets()).Load, Now: opts.Now, Resolve: resolver(reg), Folders: func(name string) (string, string) { return opts.Home.PipelineData(name), opts.Home.PipelineCache(name) },
		Finished: func(run api.Run, chain int) {
			select {
			case finished <- finishedRun{run, chain}:
			default: // the daemon is far behind: counted and logged by Serve
				dropped.Add(1)
			}
		}})
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
		opts:        opts,
		registry:    reg,
		runs:        mgr,
		listener:    ln,
		endpoint:    api.Endpoint{Address: ln.Addr().String(), Token: hex.EncodeToString(token), PID: os.Getpid(), StartedAt: now, Version: opts.Version},
		sched:       scheduler.New(opts.Grace),
		defs:        map[string]*definition{},
		stop:        make(chan struct{}),
		watches:     map[string]*fileWatch{},
		pendingRuns: map[string]int{},
		finished:    finished,
		dropped:     dropped,
	}
	d.metrics = metrics.New(metrics.Options{Self: os.Getpid(), Targets: d.metricTargets})

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

// metricTargets lists the step attempts running now, for the sampler.
func (d *Daemon) metricTargets() []metrics.Target {
	var out []metrics.Target
	for _, p := range d.runs.LiveProcesses() {
		out = append(out, metrics.Target{RunID: p.RunID, StepID: p.StepID, PID: p.PID})
	}
	return out
}

// Endpoint returns where the daemon listens and the token clients need.
func (d *Daemon) Endpoint() api.Endpoint { return d.endpoint }

// Stop asks Serve to return. It is safe to call more than once.
func (d *Daemon) Stop() { d.stopOnce.Do(func() { close(d.stop) }) }

// Serve answers API requests and fires schedules until ctx is done or Stop
// is called. It then cancels active runs, removes the endpoint file and
// returns.
func (d *Daemon) Serve(ctx context.Context) error {
	server := &http.Server{Handler: d.handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute}
	served := make(chan error, 1)
	go func() { served <- server.Serve(d.listener) }()
	d.runs.Resume()
	sampling, stopSampling := context.WithCancel(ctx)
	sampled := make(chan struct{})
	go func() { d.metrics.Run(sampling); close(sampled) }()
	defer func() { stopSampling(); <-sampled }() // no sample after Serve returns
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
			d.checkFileHooks(d.opts.Now())
			d.startPendingRuns()
			if n := d.dropped.Swap(0); n > 0 {
				d.logf("%d ended runs were not checked for on.run and notifications: the daemon was too busy", n)
			}
		case f := <-d.finished:
			d.runFinished(f)
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
				d.logf("flow %s: record missed schedule: %v", dec.Pipeline, err)
			}
			d.logf("flow %s: skipped the schedule %q for %s (missed)", dec.Pipeline, dec.Cron, stamp(dec.ScheduledFor))
		case scheduler.Fire, scheduler.CatchUp:
			trigger := api.TriggerSchedule
			if dec.Action == scheduler.CatchUp {
				trigger = api.TriggerCatchUp
			}
			run, err := d.submit(dec.Pipeline, trigger, dec.ScheduledFor)
			if err != nil {
				d.logf("flow %s: schedule %q could not start a run: %v", dec.Pipeline, dec.Cron, err)
				continue
			}
			d.logf("flow %s: schedule %q for %s -> run %s (%s)", dec.Pipeline, dec.Cron, stamp(dec.ScheduledFor), run.ID, run.Status)
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
	unchanged := old != nil && statErr == nil && !old.retry && !old.modTime.IsZero() && info.ModTime().Equal(old.modTime) && info.Size() == old.size
	missingStill := old != nil && statErr != nil && !old.retry && old.modTime.IsZero()
	cur := old
	if force || (!unchanged && !missingStill) {
		cur = &definition{checked: time.Now()}
		if old != nil {
			cur.armed, cur.scheduled, cur.named = old.armed, old.scheduled, old.named
		}
		if statErr != nil {
			cur.err = fmt.Errorf("read flow file: %w", statErr)
		} else {
			cur.modTime, cur.size = info.ModTime(), info.Size()
			cur.def, cur.plan, cur.err = readDefinition(e.Path)
			// A read that failed is tried again (a file another program held for a
			// moment). A file that fails on its content fails the same way until it
			// changes, so it is not parsed again.
			var readErr *fs.PathError
			cur.retry = errors.As(cur.err, &readErr)
			if cur.def.Name != "" {
				cur.named = cur.def.Name
			}
		}
		if cur.err == nil {
			cur.armed = cur.def.Schedules
		}
		d.defs[e.Name] = cur
		if old != nil && !unchanged {
			d.logChanged(e, old, cur)
		}
	} else {
		cur.checked = time.Now()
	}
	// The scheduler is told again only when the schedules really changed (or
	// were switched on or off). Reloading an unchanged file, as a manual
	// trigger does, must not move the clock a schedule counts from.
	want := !e.Paused && len(cur.armed) > 0
	switch {
	case want && (!cur.scheduled || old == nil || !reflect.DeepEqual(old.armed, cur.armed)):
		d.sched.Set(e.Name, cur.armed, since)
		cur.scheduled = true
	case !want && cur.scheduled:
		d.sched.Remove(e.Name)
		cur.scheduled = false
	}
	return cur
}

// checkEvery is how long a request trusts the last look at a file. The tick
// looks every second regardless, so this only spares the file system when
// Desktop and scripts ask often.
const checkEvery = 250 * time.Millisecond

// check is load for a request that shows or uses a pipeline: it looks at the
// file unless it was looked at a moment ago. The caller holds d.mu.
func (d *Daemon) check(e registry.Entry) *definition {
	if cur := d.defs[e.Name]; cur != nil && time.Since(cur.checked) < checkEvery {
		return cur
	}
	return d.load(e, d.opts.Now(), false)
}

// logChanged records that a registered file was read again because it changed.
// A file that stays broken is logged once, not at every check.
func (d *Daemon) logChanged(e registry.Entry, old, cur *definition) {
	switch {
	case cur.err != nil && old.err != nil && cur.err.Error() == old.err.Error():
		return
	case cur.err != nil:
		d.logf("flow %s: file changed and does not load: %v", e.Name, cur.err)
	case cur.def.Name != e.Name:
		d.logf("flow %s: file changed, read again (it is named %s)", e.Name, cur.def.Name)
	default:
		d.logf("flow %s: file changed, read again", e.Name)
	}
}

// maxDefinitionBytes bounds a registered definition file; the daemon reads
// these on every change and must not be made to read without limit.
const maxDefinitionBytes = 1 << 20

// resolver lets the run manager read a registered pipeline's file.
func resolver(reg *registry.Registry) runs.Resolver {
	return func(name string) (pipeline.Pipeline, string, error) {
		e, ok := reg.Get(name)
		if !ok {
			return pipeline.Pipeline{}, "", fmt.Errorf("%w: %q", registry.ErrNotFound, name)
		}
		def, _, err := readDefinition(e.Path)
		return def, workingDir(e.Path), err
	}
}

// workingDir is where the steps of a registered file run: the project root
// for a flow in <root>/.truss/flows/, else the file's own folder.
func workingDir(path string) string {
	if root, ok := project.RootOf(path); ok {
		return root
	}
	return filepath.Dir(path)
}

func readDefinition(path string) (pipeline.Pipeline, pipeline.ExecutionPlan, error) {
	info, err := os.Stat(path)
	if err != nil {
		return pipeline.Pipeline{}, pipeline.ExecutionPlan{}, fmt.Errorf("read flow file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return pipeline.Pipeline{}, pipeline.ExecutionPlan{}, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxDefinitionBytes {
		return pipeline.Pipeline{}, pipeline.ExecutionPlan{}, fmt.Errorf("%s is too large (%d bytes, the limit is %d)", path, info.Size(), maxDefinitionBytes)
	}
	def, err := pipeline.ParseFile(context.Background(), path)
	if err != nil {
		return def, pipeline.ExecutionPlan{}, err
	}
	if def.Name == "" {
		def.Name = project.FileStem(path) // spec 016 FR-008
	}
	// A flow of a project gets the project-wide env (spec 026).
	if root, ok := project.RootOf(path); ok {
		proj, err := project.Open(root)
		if err != nil {
			return def, pipeline.ExecutionPlan{}, err
		}
		def.ProjectEnv = proj.Env
	}
	if err := pipeline.Validate(def); err != nil {
		return def, pipeline.ExecutionPlan{}, err
	}
	plan, err := pipeline.Plan(def)
	return def, plan, err
}

// submit reads the definition as it is now and requests a run of it.
func (d *Daemon) submit(name, trigger string, scheduledFor time.Time) (api.Run, error) {
	return d.submitRequest(name, trigger, scheduledFor, nil, 0)
}

// submitHook requests a run started by a hook (spec 029).
func (d *Daemon) submitHook(name, trigger string, changed []string, chain int) (api.Run, error) {
	return d.submitRequest(name, trigger, time.Time{}, changed, chain)
}

func (d *Daemon) submitRequest(name, trigger string, scheduledFor time.Time, changed []string, chain int) (api.Run, error) {
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
	data, cache := d.opts.Home.PipelineData(name), d.opts.Home.PipelineCache(name)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		err = fmt.Errorf("prepare the data folder of flow %s: %w", name, err)
		if trigger != api.TriggerManual {
			_, _ = d.runs.Record(name, trigger, scheduledFor, api.RunFailed, err.Error())
		}
		return api.Run{}, err
	}
	return d.runs.Submit(runs.Request{
		Pipeline:     name,
		Definition:   cur.def,
		Dir:          workingDir(e.Path),
		ProjectDir:   workingDir(e.Path),
		DataDir:      data,
		CacheDir:     cache,
		Trigger:      trigger,
		ScheduledFor: scheduledFor,
		ChangedFiles: changed,
		Chain:        chain,
	})
}

// invalidError marks a definition that cannot run as it is.
type invalidError struct{ err error }

func (e *invalidError) Error() string { return e.err.Error() }
func (e *invalidError) Unwrap() error { return e.err }

// view builds the API form of one registered pipeline.
func (d *Daemon) view(e registry.Entry) api.Pipeline { return d.viewAs(e, d.names()) }

// viewAs is view with the names shown for every registration already worked
// out (see shownNames).
func (d *Daemon) viewAs(e registry.Entry, shown map[string]string) api.Pipeline {
	name := shown[e.Name]
	if name == "" {
		name = e.Name
	}
	p := api.Pipeline{Name: name, Path: e.Path, Paused: e.Paused, RegisteredAt: e.RegisteredAt, Schedules: []api.Schedule{}, Steps: []api.PlanStep{}}
	if name != e.Name {
		p.RegisteredName = e.Name
	}
	d.mu.Lock()
	// The file is checked on every read, so a view never lags an edit; the
	// check reads the file only when its size or time changed.
	cur := d.check(e)
	next := d.sched.Next(e.Name)
	d.mu.Unlock()

	var history []string
	if name != e.Name {
		history = []string{e.Name, name}
	} else {
		history = []string{e.Name}
	}
	p.ActiveRuns, p.LastRun = d.runs.SummaryFor(history)
	if p.LastRun != nil {
		last := showRun(shown, *p.LastRun)
		p.LastRun = &last
	}
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

// CheckAddress reports why address cannot be a daemon address: it is not
// host:port with a numeric port, or not on the loopback interface.
func CheckAddress(address string) error {
	if err := checkLoopback(address); err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(address)
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("daemon address %q: the port must be a number from 0 to 65535", address)
	}
	return nil
}

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
