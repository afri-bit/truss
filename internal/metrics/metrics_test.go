package metrics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// fakeSource returns scripted readings, one per call to host.
type fakeSource struct {
	hostErr []bool // per call: host() fails
	hosts   []hostRaw
	groupR  []map[int]procRaw
	self    []procRaw
	call    int
}

func (f *fakeSource) host() (hostRaw, error) {
	if f.call < len(f.hostErr) && f.hostErr[f.call] {
		return hostRaw{}, errors.New("read failed")
	}
	return f.hosts[min(f.call, len(f.hosts)-1)], nil
}
func (f *fakeSource) groups([]int) map[int]procRaw {
	return f.groupR[min(f.call, len(f.groupR)-1)]
}
func (f *fakeSource) process(int) (procRaw, bool) {
	r := f.self[min(f.call, len(f.self)-1)]
	f.call++ // process is read last in a sample
	return r, true
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %.3f, want %.3f", what, got, want)
	}
}

func TestSampler_ComputesSharesBetweenTwoReadings(t *testing.T) {
	c := &clock{t: time.Unix(1_000, 0)}
	src := &fakeSource{
		hosts: []hostRaw{
			{all: cpuTimes{busy: 100, total: 400}, cores: []cpuTimes{{50, 200}, {50, 200}}, memTotal: 1000, memUsed: 400, load: []float64{1, 2, 3}},
			{all: cpuTimes{busy: 300, total: 800}, cores: []cpuTimes{{200, 400}, {100, 400}}, memTotal: 1000, memUsed: 500, load: []float64{1, 2, 3}},
		},
		groupR: []map[int]procRaw{
			{42: {cpuSeconds: 10, rss: 100, processes: 2, tree: true}},
			{42: {cpuSeconds: 11, rss: 300, processes: 3, tree: true}},
		},
		self: []procRaw{{cpuSeconds: 5, rss: 50, processes: 1}, {cpuSeconds: 5.5, rss: 60, processes: 1}},
	}
	s := newSampler(Options{Self: 7, Now: c.now, Targets: func() []Target { return []Target{{RunID: "r1", StepID: "build", PID: 42}} }}, src, "")

	s.Sample()
	first := s.Snapshot()
	if !first.Supported || first.Host == nil || first.Host.CPUPercent != 0 || len(first.History) != 0 {
		t.Fatalf("first sample has no delta yet: %+v", first)
	}

	c.t = c.t.Add(2 * time.Second)
	s.Sample()
	m := s.Snapshot()
	near(t, "host cpu", m.Host.CPUPercent, 50)     // 200 busy of 400
	near(t, "core 0", m.Host.Cores[0], 75)         // 150 of 200
	near(t, "core 1", m.Host.Cores[1], 25)         // 50 of 200
	near(t, "step cpu", m.Steps[0].CPUPercent, 25) // 1 s of CPU in 2 s on 2 cores
	near(t, "daemon cpu", m.Daemon.CPUPercent, 12.5)
	if m.Host.MemoryUsedBytes != 500 || m.Host.MemoryTotalBytes != 1000 {
		t.Errorf("memory = %d of %d", m.Host.MemoryUsedBytes, m.Host.MemoryTotalBytes)
	}
	if st := m.Steps[0]; st.RunID != "r1" || st.StepID != "build" || st.PID != 42 || st.RSSBytes != 300 || st.Processes != 3 || !st.Tree {
		t.Errorf("step = %+v", st)
	}
	if m.Daemon.PID != 7 || m.Daemon.RSSBytes != 60 {
		t.Errorf("daemon = %+v", m.Daemon)
	}
	if len(m.History) != 1 || m.History[0].CPUPercent != m.Host.CPUPercent {
		t.Errorf("history = %+v", m.History)
	}
}

func TestSampler_KeepsOnlyRecentHistory(t *testing.T) {
	c := &clock{t: time.Unix(1_000, 0)}
	var hosts []hostRaw
	for i := 0; i < 20; i++ {
		hosts = append(hosts, hostRaw{all: cpuTimes{busy: float64(i), total: float64(10 * i)}})
	}
	src := &fakeSource{hosts: hosts, groupR: []map[int]procRaw{{}}, self: []procRaw{{}}}
	s := newSampler(Options{Interval: time.Second, Keep: 5 * time.Second, Now: c.now}, src, "")
	for i := 0; i < 20; i++ {
		s.Sample()
		c.t = c.t.Add(time.Second)
	}
	h := s.Snapshot().History
	if len(h) != 6 { // samples within the last 5 s, inclusive
		t.Fatalf("history has %d points, want 6", len(h))
	}
	for i := 1; i < len(h); i++ {
		if !h[i].At.After(h[i-1].At) {
			t.Fatal("history is not oldest first")
		}
	}
}

func TestSampler_LeavesOutStepsWhoseProcessesAreGone(t *testing.T) {
	src := &fakeSource{hosts: []hostRaw{{}}, groupR: []map[int]procRaw{{}}, self: []procRaw{{}}}
	s := newSampler(Options{Targets: func() []Target { return []Target{{RunID: "r", StepID: "gone", PID: 9}} }}, src, "")
	s.Sample()
	if got := s.Snapshot().Steps; len(got) != 0 {
		t.Fatalf("steps = %+v", got)
	}
}

func TestSampler_Unsupported(t *testing.T) {
	s := newSampler(Options{Self: 3}, nil, "not here")
	s.Run(context.Background()) // returns at once
	s.Sample()
	m := s.Snapshot()
	if m.Supported || m.Reason != "not here" || m.Host != nil || m.Daemon.PID != 3 || m.Steps == nil || m.History == nil {
		t.Fatalf("snapshot = %+v", m)
	}
}

func TestSampler_SnapshotIsACopy(t *testing.T) {
	src := &fakeSource{hosts: []hostRaw{{cores: []cpuTimes{{1, 2}}}}, groupR: []map[int]procRaw{{}}, self: []procRaw{{}}}
	s := newSampler(Options{}, src, "")
	s.Sample()
	a := s.Snapshot()
	a.Host.Cores[0] = 99
	if s.Snapshot().Host.Cores[0] == 99 {
		t.Fatal("Snapshot shares its slices")
	}
}

func TestSampler_RunStopsWithItsContext(t *testing.T) {
	src := &fakeSource{hosts: []hostRaw{{}}, groupR: []map[int]procRaw{{}}, self: []procRaw{{}}}
	s := newSampler(Options{Interval: 10 * time.Millisecond}, src, "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestNew_ReadsThisMachine is a smoke test of the real source for the OS the
// tests run on: Linux and Windows report figures, others report why not.
func TestNew_ReadsThisMachine(t *testing.T) {
	s := New(Options{Self: os.Getpid(), Interval: 50 * time.Millisecond})
	s.Sample()
	time.Sleep(100 * time.Millisecond)
	s.Sample()
	m := s.Snapshot()
	switch runtime.GOOS {
	case "linux", "windows":
		if !m.Supported || m.Host == nil {
			t.Fatalf("snapshot = %+v", m)
		}
		if m.Host.MemoryTotalBytes == 0 || m.Host.MemoryUsedBytes == 0 || m.Host.MemoryUsedBytes > m.Host.MemoryTotalBytes {
			t.Errorf("memory = %d of %d", m.Host.MemoryUsedBytes, m.Host.MemoryTotalBytes)
		}
		if len(m.Host.Cores) == 0 {
			t.Error("no cores")
		}
		if m.Daemon.RSSBytes == 0 {
			t.Errorf("own process = %+v", m.Daemon)
		}
	default:
		if m.Supported || m.Reason == "" {
			t.Fatalf("snapshot = %+v", m)
		}
	}
}

func TestSampler_HistoryStaysBoundedWhenTheClockGoesBack(t *testing.T) {
	c := &clock{t: time.Unix(100_000, 0)}
	src := &fakeSource{hosts: []hostRaw{{}}, groupR: []map[int]procRaw{{}}, self: []procRaw{{}}}
	s := newSampler(Options{Interval: time.Second, Keep: 5 * time.Second, Now: c.now}, src, "")
	s.Sample()
	c.t = c.t.Add(-time.Hour) // the wall clock is set back
	for i := 0; i < 50; i++ {
		c.t = c.t.Add(time.Second)
		s.Sample()
	}
	if n := len(s.Snapshot().History); n > 6 {
		t.Fatalf("history has %d points after a clock step, want at most 6", n)
	}
}

func TestSampler_DeltasNeedTwoGoodReadingsOfTheSameThing(t *testing.T) {
	c := &clock{t: time.Unix(1_000, 0)}
	targets := []Target{{RunID: "r1", StepID: "a", PID: 42}}
	src := &fakeSource{
		hostErr: []bool{false, true, false, false},
		hosts: []hostRaw{
			{all: cpuTimes{busy: 0, total: 100}, cores: []cpuTimes{{0, 50}, {0, 50}}},
			{}, // failed read
			{all: cpuTimes{busy: 50, total: 200}, cores: []cpuTimes{{25, 100}, {25, 100}}},
			{all: cpuTimes{busy: 100, total: 300}, cores: []cpuTimes{{50, 150}, {50, 150}}},
		},
		groupR: []map[int]procRaw{
			{42: {cpuSeconds: 10}},
			{42: {cpuSeconds: 12}},
			{42: {cpuSeconds: 1}}, // pid 42 now belongs to another step: counters went back
			{42: {cpuSeconds: 3}},
		},
		self: []procRaw{{cpuSeconds: 1}, {cpuSeconds: 2}, {cpuSeconds: 3}, {cpuSeconds: 4}},
	}
	s := newSampler(Options{Now: c.now, Targets: func() []Target { return targets }}, src, "")
	step := func() float64 { m := s.Snapshot(); return m.Steps[0].CPUPercent }

	s.Sample() // baseline
	c.t = c.t.Add(time.Second)
	s.Sample() // host read fails: no host, no history; the step still has a delta
	if m := s.Snapshot(); m.Host != nil || len(m.History) != 0 {
		t.Fatalf("a failed host read must show no host figures: %+v", m.Host)
	}
	near(t, "step cpu after 2 s of CPU in 1 s on 2 cores", step(), 100)

	targets = []Target{{RunID: "r2", StepID: "b", PID: 42}} // the pid was reused
	c.t = c.t.Add(time.Second)
	s.Sample() // the host has no good previous reading; the step is new
	m := s.Snapshot()
	if m.Host == nil || m.Host.CPUPercent != 0 || len(m.History) != 0 {
		t.Fatalf("host after a failed read must start afresh: %+v", m.Host)
	}
	if step() != 0 {
		t.Fatalf("a reused pid in another step must not reuse the old counters: %v", step())
	}
	c.t = c.t.Add(time.Second)
	s.Sample()
	near(t, "host cpu", s.Snapshot().Host.CPUPercent, 50)
	near(t, "step cpu of the new step", step(), 100)
}

// writeStat writes a /proc/<pid>/stat line in dir.
func writeStat(t *testing.T, dir string, pid, pgrp int, utime, cutime int) {
	t.Helper()
	d := filepath.Join(dir, strconv.Itoa(pid))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf("%d (x) S 1 %d %d 0 -1 0 0 0 0 0 %d 0 %d 0 20 0 1 0 1 0 10 0\n", pid, pgrp, pgrp, utime, cutime)
	if err := os.WriteFile(filepath.Join(d, "stat"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
}
