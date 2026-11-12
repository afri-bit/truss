// Package runs executes pipeline runs for the daemon and keeps their history.
//
// The Manager owns the queue. It decides when a requested run starts (the
// global limit on parallel runs and each pipeline's overlap policy), runs it
// through the engine, follows its steps, and stores the record and the run
// log under the TRUSS home so they survive a daemon restart.
package runs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/engine"
	"github.com/afri-bit/truss/internal/executor"
	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/pipeline"
	"github.com/afri-bit/truss/internal/runlog"
)

// ErrNotFound means no run has the given id.
var ErrNotFound = errors.New("run not found")

// ErrClosed means the manager is shutting down and accepts no new runs.
var ErrClosed = errors.New("the daemon is shutting down")

// Defaults for Options.
const (
	DefaultMaxParallel = 4
	DefaultKeep        = 200
	logChunk           = 256 << 10
)

// Options configures a Manager.
type Options struct {
	// Dir is the directory that holds one folder per run.
	Dir string
	// MaxParallel is the most runs that execute at once, across pipelines.
	MaxParallel int
	// Keep is how many finished runs stay in the history.
	Keep int
	// Env is the environment every step starts from ("KEY=value" entries).
	Env []string
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Request asks for one run of a registered pipeline.
type Request struct {
	// Pipeline is the registered name.
	Pipeline string
	// Definition is the validated definition to run, read when the run was requested.
	Definition pipeline.Pipeline
	// Dir is the directory the run starts in.
	Dir string
	// DataDir and CacheDir, when set, are handed to steps as TRUSS_DATA_DIR
	// and TRUSS_CACHE_DIR. The caller creates them.
	DataDir  string
	CacheDir string
	Trigger  string
	// ScheduledFor is the time a schedule named; zero for manual runs.
	ScheduledFor time.Time
}

type record struct {
	run       api.Run
	req       Request
	cancel    context.CancelFunc
	cancelled bool            // someone asked for the run to end
	stopped   map[string]bool // steps TRUSS stopped after a failure
}

// Manager queues, executes and remembers runs. It is safe for concurrent use.
type Manager struct {
	opts Options

	mu      sync.Mutex
	records map[string]*record
	order   []string // run ids, oldest first
	queue   []string // queued run ids, oldest first
	// lastCreated is the newest creation time handed out. New runs are always
	// created after it, so history order never depends on clock resolution.
	lastCreated time.Time
	closed      bool
	wg          sync.WaitGroup
}

// Open returns a Manager with the history found in opts.Dir. Runs that were
// active when the previous daemon stopped are closed as failed.
func Open(opts Options) (*Manager, error) {
	if opts.MaxParallel < 1 {
		opts.MaxParallel = DefaultMaxParallel
	}
	if opts.Keep < 1 {
		opts.Keep = DefaultKeep
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("prepare run history: %w", err)
	}
	m := &Manager{opts: opts, records: map[string]*record{}}
	dirs, err := os.ReadDir(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("read run history: %w", err)
	}
	for _, d := range dirs {
		var run api.Run
		if !d.IsDir() || home.ReadJSON(filepath.Join(opts.Dir, d.Name(), "run.json"), &run) != nil || run.ID != d.Name() {
			continue
		}
		rec := &record{run: run}
		if run.Active() {
			m.finish(rec, api.RunFailed, "the daemon stopped while this run was active")
			_ = m.save(rec)
		}
		m.records[run.ID] = rec
		m.order = append(m.order, run.ID)
		if run.CreatedAt.After(m.lastCreated) {
			m.lastCreated = run.CreatedAt
		}
	}
	sort.Slice(m.order, func(i, j int) bool {
		a, b := m.records[m.order[i]].run, m.records[m.order[j]].run
		if a.CreatedAt.Equal(b.CreatedAt) {
			return a.ID < b.ID
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
	m.prune()
	return m, nil
}

// Submit requests a run. Depending on the pipeline's overlap policy the
// returned run is queued, already running, or recorded as skipped.
func (m *Manager) Submit(req Request) (api.Run, error) {
	plan, err := pipeline.Plan(req.Definition)
	if err != nil {
		return api.Run{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return api.Run{}, ErrClosed
	}
	rec := m.newRecord(req.Pipeline, req.Trigger, req.ScheduledFor)
	rec.req = req
	for _, s := range plan.Graph {
		rec.run.Steps = append(rec.run.Steps, api.Step{ID: s.ID, Needs: s.Needs, Status: api.StepPending})
	}
	if req.Definition.Overlap == pipeline.OverlapSkip && m.activeFor(req.Pipeline) > 0 {
		m.finish(rec, api.RunSkipped, "another run of this pipeline was still active")
		return m.add(rec)
	}
	rec.run.Status = api.RunQueued
	m.queue = append(m.queue, rec.run.ID)
	run, err := m.add(rec)
	if err != nil {
		m.queue = m.queue[:len(m.queue)-1]
		return api.Run{}, err
	}
	m.dispatch()
	return m.records[run.ID].run.Copy(), nil
}

// Record stores a run that never executes, such as a scheduled time that was
// missed (api.RunSkipped) or a definition that no longer loads (api.RunFailed).
func (m *Manager) Record(name, trigger string, scheduledFor time.Time, status, reason string) (api.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.newRecord(name, trigger, scheduledFor)
	m.finish(rec, status, reason)
	return m.add(rec)
}

// Cancel ends a queued or running run. Cancelling a finished run changes nothing.
func (m *Manager) Cancel(id string) (api.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return api.Run{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	switch rec.run.Status {
	case api.RunQueued:
		m.unqueue(id)
		m.finish(rec, api.RunCancelled, "cancelled before it started")
		_ = m.save(rec)
	case api.RunRunning:
		rec.cancelled = true
		rec.cancel() // the run's goroutine records the outcome
	}
	return rec.run.Copy(), nil
}

// Get returns one run.
func (m *Manager) Get(id string) (api.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.records[id]
	if !ok {
		return api.Run{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return rec.run.Copy(), nil
}

// List returns runs, newest first. An empty name lists every pipeline; a
// limit below 1 means no limit.
func (m *Manager) List(name string, limit int) []api.Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := []api.Run{}
	for i := len(m.order) - 1; i >= 0; i-- {
		run := m.records[m.order[i]].run
		if name != "" && run.Pipeline != name {
			continue
		}
		list = append(list, run.Copy())
		if limit > 0 && len(list) == limit {
			break
		}
	}
	return list
}

// Summary returns the number of active runs of a pipeline and its newest
// run that actually executed or is about to, if any.
func (m *Manager) Summary(name string) (active int, last *api.Run) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.order) - 1; i >= 0; i-- {
		run := m.records[m.order[i]].run
		if run.Pipeline != name {
			continue
		}
		if run.Active() {
			active++
		}
		if last == nil {
			c := run.Copy()
			last = &c
		}
	}
	return active, last
}

// Counts returns how many runs are running and how many are queued.
func (m *Manager) Counts() (running, queued int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range m.records {
		if rec.run.Status == api.RunRunning {
			running++
		}
	}
	return running, len(m.queue)
}

// MaxParallel returns the limit on runs that execute at once.
func (m *Manager) MaxParallel() int { return m.opts.MaxParallel }

// Log returns run-log bytes starting at offset, the offset to ask for next,
// and the run's status at that moment. An empty chunk for a finished run
// means the log is complete.
func (m *Manager) Log(id string, offset int64) ([]byte, int64, string, error) {
	run, err := m.Get(id)
	if err != nil {
		return nil, 0, "", err
	}
	f, err := os.Open(m.logPath(id)) // #nosec G304 -- the id was looked up in the history
	if os.IsNotExist(err) {
		return nil, 0, run.Status, nil // skipped and queued runs have no log
	}
	if err != nil {
		return nil, 0, "", fmt.Errorf("open run log: %w", err)
	}
	defer func() { _ = f.Close() }()
	if offset < 0 {
		offset = 0
	}
	buf := make([]byte, logChunk)
	n, err := f.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, "", fmt.Errorf("read run log: %w", err)
	}
	return buf[:n], offset + int64(n), run.Status, nil
}

// Close cancels every active run and waits for them to end.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, id := range m.queue {
		rec := m.records[id]
		m.finish(rec, api.RunCancelled, "the daemon stopped")
		_ = m.save(rec)
	}
	m.queue = nil
	for _, rec := range m.records {
		if rec.run.Status == api.RunRunning {
			rec.cancelled = true
			rec.cancel()
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}

// newRecord creates a record with a fresh id. The caller holds m.mu.
func (m *Manager) newRecord(name, trigger string, scheduledFor time.Time) *record {
	// A coarse clock (Windows) can return the same instant twice; the history
	// is sorted by creation time, so each run gets a strictly later one.
	now := m.opts.Now()
	if !now.After(m.lastCreated) {
		now = m.lastCreated.Add(time.Nanosecond)
	}
	m.lastCreated = now
	rec := &record{stopped: map[string]bool{}}
	rec.run = api.Run{Pipeline: name, Trigger: trigger, CreatedAt: now, Steps: []api.Step{}}
	if !scheduledFor.IsZero() {
		rec.run.ScheduledFor = &scheduledFor
	}
	for {
		var b [3]byte
		_, _ = rand.Read(b[:])
		rec.run.ID = now.Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
		if _, taken := m.records[rec.run.ID]; !taken {
			return rec
		}
	}
}

// add stores a new record. The caller holds m.mu.
func (m *Manager) add(rec *record) (api.Run, error) {
	if err := os.MkdirAll(filepath.Join(m.opts.Dir, rec.run.ID), 0o700); err != nil {
		return api.Run{}, fmt.Errorf("store run: %w", err)
	}
	if err := m.save(rec); err != nil {
		return api.Run{}, err
	}
	m.records[rec.run.ID] = rec
	m.order = append(m.order, rec.run.ID)
	m.prune()
	return rec.run.Copy(), nil
}

// finish closes a run and every step that is still open. The caller holds m.mu.
func (m *Manager) finish(rec *record, status, reason string) {
	now := m.opts.Now()
	rec.run.Status = status
	rec.run.Reason = reason
	rec.run.FinishedAt = &now
	for i := range rec.run.Steps {
		step := &rec.run.Steps[i]
		switch step.Status {
		case api.StepPending:
			step.Status = api.StepSkipped
		case api.StepRunning:
			step.Status = api.StepCancelled
			step.FinishedAt = &now
		}
	}
}

func (m *Manager) save(rec *record) error {
	return home.WriteJSON(filepath.Join(m.opts.Dir, rec.run.ID, "run.json"), rec.run)
}

func (m *Manager) logPath(id string) string {
	return filepath.Join(m.opts.Dir, id, "run.log")
}

// activeFor counts queued and running runs of a pipeline. The caller holds m.mu.
func (m *Manager) activeFor(name string) int {
	n := 0
	for _, rec := range m.records {
		if rec.run.Pipeline == name && rec.run.Active() {
			n++
		}
	}
	return n
}

func (m *Manager) unqueue(id string) {
	for i, q := range m.queue {
		if q == id {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

// dispatch starts queued runs while the limits allow it, oldest first. A
// pipeline runs one run at a time unless its overlap policy is parallel.
// The caller holds m.mu.
func (m *Manager) dispatch() {
	running := map[string]int{}
	total := 0
	for _, rec := range m.records {
		if rec.run.Status == api.RunRunning {
			running[rec.run.Pipeline]++
			total++
		}
	}
	var waiting []string
	for _, id := range m.queue {
		rec := m.records[id]
		name := rec.run.Pipeline
		parallel := rec.req.Definition.Overlap == pipeline.OverlapParallel
		if m.closed || total >= m.opts.MaxParallel || (running[name] > 0 && !parallel) {
			waiting = append(waiting, id)
			continue
		}
		running[name]++
		total++
		m.start(rec)
	}
	m.queue = waiting
}

// start marks a run as running and executes it. The caller holds m.mu.
func (m *Manager) start(rec *record) {
	ctx, cancel := context.WithCancel(context.Background())
	now := m.opts.Now()
	rec.cancel = cancel
	rec.run.Status = api.RunRunning
	rec.run.StartedAt = &now
	_ = m.save(rec)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		m.execute(ctx, rec)
	}()
}

// execute runs one pipeline through the engine and records the outcome.
func (m *Manager) execute(ctx context.Context, rec *record) {
	id, req := rec.run.ID, rec.req
	out := io.Discard
	f, logErr := os.OpenFile(m.logPath(id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- path is built from the run id
	if logErr == nil {
		out = f
		defer func() { _ = f.Close() }()
	}
	log := runlog.New(out, m.opts.Now)
	log.Setup(runlog.OSName(runtime.GOOS), runtime.GOARCH, executor.Shell(), req.Dir)

	env := append(append([]string(nil), m.opts.Env...),
		"TRUSS_RUN_ID="+id, "TRUSS_PIPELINE="+req.Pipeline, "TRUSS_TRIGGER="+req.Trigger)
	if req.DataDir != "" {
		env = append(env, "TRUSS_DATA_DIR="+req.DataDir)
	}
	if req.CacheDir != "" {
		env = append(env, "TRUSS_CACHE_DIR="+req.CacheDir)
	}
	result, err := engine.Run(ctx, req.Definition, req.Dir, env, &follower{m: m, rec: rec, log: log})

	m.mu.Lock()
	defer m.mu.Unlock()
	started := 0
	for _, s := range rec.run.Steps {
		if s.StartedAt != nil {
			started++
		}
	}
	switch {
	case rec.cancelled || ctx.Err() != nil:
		reason := "cancelled"
		if m.closed {
			reason = "the daemon stopped"
		}
		log.SummaryCancelled(started)
		m.finish(rec, api.RunCancelled, reason)
	case err != nil:
		log.SummaryFailed(started, err.Error())
		m.finish(rec, api.RunFailed, err.Error())
	case result.Status != pipeline.StatusSucceeded:
		log.SummaryFailed(started, result.Failure())
		m.finish(rec, api.RunFailed, result.Failure())
	default:
		log.SummarySucceeded(started)
		m.finish(rec, api.RunSucceeded, "")
	}
	_ = m.save(rec)
	m.dispatch()
}

// prune drops the oldest finished runs beyond the history limit. The caller
// holds m.mu.
func (m *Manager) prune() {
	extra := len(m.order) - m.opts.Keep
	if extra <= 0 {
		return
	}
	kept := m.order[:0:0]
	for _, id := range m.order {
		rec := m.records[id]
		if extra > 0 && !rec.run.Active() {
			extra--
			delete(m.records, id)
			_ = os.RemoveAll(filepath.Join(m.opts.Dir, id))
			continue
		}
		kept = append(kept, id)
	}
	m.order = kept
}

// follower turns engine events into run-log lines and step states.
type follower struct {
	m   *Manager
	rec *record
	log *runlog.Log
}

func (f *follower) step(id string, change func(*api.Step)) {
	f.m.mu.Lock()
	defer f.m.mu.Unlock()
	for i := range f.rec.run.Steps {
		if f.rec.run.Steps[i].ID == id {
			change(&f.rec.run.Steps[i])
			_ = f.m.save(f.rec)
			return
		}
	}
}

func (f *follower) StepStarted(id string) {
	f.log.StepStarted(id)
	now := f.m.opts.Now()
	f.step(id, func(s *api.Step) { s.Status, s.StartedAt = api.StepRunning, &now })
}

func (f *follower) StepOutput(id string, chunk []byte) { f.log.StepOutput(id, chunk) }

func (f *follower) StepStopped(id, failedID string) {
	f.log.StepStopped(id, failedID)
	f.m.mu.Lock()
	f.rec.stopped[id] = true
	f.m.mu.Unlock()
}

func (f *follower) StepFinished(id string, exitCode *int, startErr error) {
	f.log.StepFinished(id, exitCode, startErr)
	now := f.m.opts.Now()
	f.step(id, func(s *api.Step) {
		s.FinishedAt = &now
		if s.StartedAt == nil {
			s.StartedAt = &now
		}
		s.ExitCode = exitCode
		switch {
		case f.rec.cancelled:
			s.Status = api.StepCancelled
		case f.rec.stopped[id]:
			s.Status = api.StepStopped
		case startErr != nil:
			s.Status, s.Error, s.ExitCode = api.StepFailed, startErr.Error(), nil
		case exitCode == nil || *exitCode != 0:
			s.Status = api.StepFailed
		default:
			s.Status = api.StepSucceeded
		}
	})
}
