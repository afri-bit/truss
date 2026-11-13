// Package metrics samples the machine's CPU and memory, and the processes of
// running steps, for the daemon's GET /metrics. It reads what the operating
// system already exposes and starts no process. Linux and Windows are
// supported; elsewhere Snapshot says so and reports nothing.
package metrics

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/afri-bit/truss/internal/api"
)

// Target is a running step whose processes are measured.
type Target struct {
	RunID  string
	StepID string
	PID    int
}

// Options configures a Sampler. Zero values take the defaults.
type Options struct {
	// Interval between samples; default 2 seconds.
	Interval time.Duration
	// Keep is how much host history to remember; default 10 minutes.
	Keep time.Duration
	// Targets lists the running steps at sample time. Nil means none.
	Targets func() []Target
	// Self is the daemon's own process id.
	Self int
	// Now is the clock; default time.Now.
	Now func() time.Time
}

// cpuTimes are cumulative CPU times in one consistent unit.
type cpuTimes struct{ busy, total float64 }

// hostRaw is one cumulative reading of the machine.
type hostRaw struct {
	all      cpuTimes
	cores    []cpuTimes
	memTotal uint64
	memUsed  uint64
	load     []float64
}

// procRaw is one cumulative reading of a process or a step's process group.
type procRaw struct {
	cpuSeconds float64
	rss        uint64
	threads    int
	processes  int
	tree       bool
}

// source reads the operating system. Each OS has one; nil means unsupported.
type source interface {
	host() (hostRaw, error)
	// groups reads each step's processes, keyed by the step's process id.
	// A pid whose processes are gone is left out.
	groups(pids []int) map[int]procRaw
	// process reads one process.
	process(pid int) (procRaw, bool)
}

// Sampler keeps the latest sample and a short host history. It is safe for
// concurrent use: Run samples, Snapshot reads.
type Sampler struct {
	opts Options
	src  source
	why  string // why src is nil

	mu sync.Mutex
	at time.Time
	// The previous readings, for deltas. A delta needs two good readings of the
	// same thing: the host, the daemon, or one attempt of one step (a reused pid
	// in another step or run starts afresh).
	host    hostRaw
	hostOK  bool
	procs   map[Target]procRaw
	self    procRaw
	selfOK  bool
	cores   int // logical cores in the last good host reading
	snap    api.Metrics
	history []api.HostPoint
}

// New returns a Sampler for this operating system.
func New(opts Options) *Sampler {
	src, why := newSource()
	return newSampler(opts, src, why)
}

func newSampler(opts Options, src source, why string) *Sampler {
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	if opts.Keep <= 0 {
		opts.Keep = 10 * time.Minute
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Targets == nil {
		opts.Targets = func() []Target { return nil }
	}
	s := &Sampler{opts: opts, src: src, why: why}
	s.snap = api.Metrics{Supported: src != nil, OS: runtime.GOOS, Reason: why, IntervalSeconds: opts.Interval.Seconds(), Daemon: api.ProcessMetrics{PID: opts.Self}, Steps: []api.StepMetrics{}, History: []api.HostPoint{}}
	return s
}

// Run samples until ctx ends. The first figures are ready after a short
// warm-up, because CPU use is measured between two readings.
func (s *Sampler) Run(ctx context.Context) {
	if s.src == nil {
		return
	}
	s.Sample()
	warm := time.NewTimer(min(s.opts.Interval, 500*time.Millisecond))
	select {
	case <-ctx.Done():
		warm.Stop()
		return
	case <-warm.C:
	}
	s.Sample()
	t := time.NewTicker(s.opts.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sample()
		}
	}
}

// Sample takes one reading now. Run calls it; tests call it directly.
func (s *Sampler) Sample() {
	if s.src == nil {
		return
	}
	targets := s.opts.Targets()
	pids := make([]int, 0, len(targets))
	for _, t := range targets {
		if t.PID > 0 {
			pids = append(pids, t.PID)
		}
	}
	host, err := s.src.host()
	groups := s.src.groups(pids)
	self, selfOK := s.src.process(s.opts.Self)
	now := s.opts.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	wall := now.Sub(s.at).Seconds()
	// Shares are of the whole machine: use this reading's core count, else the last good one.
	cores := len(host.cores)
	if err != nil || cores == 0 {
		cores = s.cores
	}
	if cores == 0 {
		cores = runtime.NumCPU()
	}
	if err == nil && len(host.cores) > 0 {
		s.cores = len(host.cores)
	}
	share := func(prev, cur procRaw, had bool) float64 {
		if !had || wall <= 0 || cur.cpuSeconds < prev.cpuSeconds {
			return 0
		}
		return clamp((cur.cpuSeconds - prev.cpuSeconds) / wall / float64(cores) * 100)
	}

	snap := api.Metrics{Supported: true, OS: runtime.GOOS, IntervalSeconds: s.opts.Interval.Seconds(), Steps: []api.StepMetrics{}}
	stamp := now.UTC()
	snap.SampledAt = &stamp
	if err == nil {
		h := &api.HostMetrics{MemoryTotalBytes: host.memTotal, MemoryUsedBytes: host.memUsed, Load: host.load, Cores: make([]float64, len(host.cores))}
		if s.hostOK {
			h.CPUPercent = busy(s.host.all, host.all)
			for i, c := range host.cores {
				if i < len(s.host.cores) {
					h.Cores[i] = busy(s.host.cores[i], c)
				}
			}
			s.history = append(s.history, api.HostPoint{At: stamp, CPUPercent: h.CPUPercent, MemoryUsedBytes: host.memUsed})
			cut := 0
			for cut < len(s.history) && now.Sub(s.history[cut].At) > s.opts.Keep {
				cut++
			}
			// Also bound the length: a wall clock set back would otherwise keep every point.
			if most := int(s.opts.Keep/s.opts.Interval) + 1; len(s.history)-cut > most {
				cut = len(s.history) - most
			}
			s.history = s.history[cut:]
		}
		snap.Host = h
	}
	s.host, s.hostOK = host, err == nil

	snap.Daemon = api.ProcessMetrics{PID: s.opts.Self}
	if selfOK {
		snap.Daemon = processMetrics(s.opts.Self, self, share(s.self, self, s.selfOK))
	}
	s.self, s.selfOK = self, selfOK

	seen := map[Target]procRaw{}
	for _, t := range targets {
		cur, ok := groups[t.PID]
		if !ok {
			continue
		}
		prev, had := s.procs[t]
		seen[t] = cur
		snap.Steps = append(snap.Steps, api.StepMetrics{RunID: t.RunID, StepID: t.StepID, ProcessMetrics: processMetrics(t.PID, cur, share(prev, cur, had))})
	}
	sort.SliceStable(snap.Steps, func(i, j int) bool { return snap.Steps[i].RunID < snap.Steps[j].RunID })
	s.procs = seen
	s.at = now
	snap.History = append([]api.HostPoint(nil), s.history...)
	s.snap = snap
}

// Snapshot returns the latest sample. Its slices are the caller's to keep.
func (s *Sampler) Snapshot() api.Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.snap
	out.Steps = append([]api.StepMetrics{}, s.snap.Steps...)
	out.History = append([]api.HostPoint{}, s.snap.History...)
	if s.snap.Host != nil {
		h := *s.snap.Host
		h.Cores = append([]float64(nil), h.Cores...)
		h.Load = append([]float64(nil), h.Load...)
		out.Host = &h
	}
	return out
}

func processMetrics(pid int, r procRaw, cpu float64) api.ProcessMetrics {
	return api.ProcessMetrics{PID: pid, CPUPercent: cpu, RSSBytes: r.rss, Threads: r.threads, Processes: r.processes, Tree: r.tree}
}

// busy is the busy share between two readings, in percent.
func busy(prev, cur cpuTimes) float64 {
	total := cur.total - prev.total
	if total <= 0 {
		return 0
	}
	return clamp((cur.busy - prev.busy) / total * 100)
}

func clamp(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	}
	return v
}
