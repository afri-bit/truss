package metrics

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// clockTicks is USER_HZ, the unit of CPU times in /proc. It is 100 on every
// Linux architecture Go supports.
const clockTicks = 100

func newSource() (source, string) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		return nil, "/proc is not available"
	}
	return procfs{root: "/proc", page: uint64(os.Getpagesize())}, "" // #nosec G115 -- the page size is positive
}

// procfs reads a Linux /proc tree. root is "/proc" in production and a
// fixture directory in tests.
type procfs struct {
	root string
	page uint64
}

func (p procfs) host() (hostRaw, error) {
	var h hostRaw
	stat, err := os.ReadFile(filepath.Join(p.root, "stat"))
	if err != nil {
		return h, err
	}
	sc := bufio.NewScanner(bytes.NewReader(stat))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		// user nice system idle iowait irq softirq steal; guest time is already in user.
		var vals [8]float64
		for i := 0; i < 8 && i+1 < len(f); i++ {
			vals[i], _ = strconv.ParseFloat(f[i+1], 64)
		}
		total := 0.0
		for _, v := range vals {
			total += v
		}
		t := cpuTimes{busy: total - vals[3] - vals[4], total: total}
		if f[0] == "cpu" {
			h.all = t
		} else {
			h.cores = append(h.cores, t)
		}
	}
	if mem, err := os.ReadFile(filepath.Join(p.root, "meminfo")); err == nil {
		kb := map[string]uint64{}
		for _, line := range strings.Split(string(mem), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				v, _ := strconv.ParseUint(f[1], 10, 64)
				kb[strings.TrimSuffix(f[0], ":")] = v
			}
		}
		h.memTotal = kb["MemTotal"] * 1024
		avail, ok := kb["MemAvailable"]
		if !ok { // kernels before 3.14
			avail = kb["MemFree"] + kb["Buffers"] + kb["Cached"]
		}
		if avail*1024 <= h.memTotal {
			h.memUsed = h.memTotal - avail*1024
		}
	}
	if load, err := os.ReadFile(filepath.Join(p.root, "loadavg")); err == nil {
		f := strings.Fields(string(load))
		for i := 0; i < 3 && i < len(f); i++ {
			v, _ := strconv.ParseFloat(f[i], 64)
			h.load = append(h.load, v)
		}
	}
	return h, nil
}

// stat is the part of /proc/<pid>/stat that metrics uses.
type stat struct {
	pgrp    int
	ticks   float64 // utime + stime
	reaped  float64 // cutime + cstime: CPU of children this process has waited for
	threads int
	rssPage uint64
}

func (p procfs) readStat(dir string) (stat, bool) {
	raw, err := os.ReadFile(filepath.Join(p.root, dir, "stat")) // #nosec G304 -- a /proc/<pid>/stat file
	if err != nil {
		return stat{}, false
	}
	// The command name is in parentheses and may contain spaces or ")".
	end := bytes.LastIndexByte(raw, ')')
	if end < 0 {
		return stat{}, false
	}
	f := strings.Fields(string(raw[end+1:]))
	// f[0] is field 3 (state); field n is f[n-3].
	if len(f) < 22 {
		return stat{}, false
	}
	var s stat
	s.pgrp, _ = strconv.Atoi(f[2])
	ut, _ := strconv.ParseFloat(f[11], 64)
	st, _ := strconv.ParseFloat(f[12], 64)
	s.ticks = ut + st
	cut, _ := strconv.ParseFloat(f[13], 64)
	cst, _ := strconv.ParseFloat(f[14], 64)
	s.reaped = cut + cst
	s.threads, _ = strconv.Atoi(f[17])
	s.rssPage, _ = strconv.ParseUint(f[21], 10, 64)
	return s, true
}

// groups sums every process in each step's process group. The executor makes
// each step's shell the leader of its own group, so the group id is its pid.
func (p procfs) groups(pids []int) map[int]procRaw {
	out := map[int]procRaw{}
	if len(pids) == 0 {
		return out
	}
	want := map[int]bool{}
	for _, pid := range pids {
		want[pid] = true
	}
	entries, err := os.ReadDir(p.root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		s, ok := p.readStat(e.Name())
		if !ok || !want[s.pgrp] {
			continue
		}
		g := out[s.pgrp]
		// A child's CPU moves into its parent's reaped time when it exits, so the
		// group total keeps short-lived children (compilers, test binaries) and
		// does not fall when one of them ends.
		g.cpuSeconds += (s.ticks + s.reaped) / clockTicks
		g.rss += s.rssPage * p.page
		g.threads += s.threads
		g.processes++
		g.tree = true
		out[s.pgrp] = g
	}
	return out
}

func (p procfs) process(pid int) (procRaw, bool) {
	if pid <= 0 {
		return procRaw{}, false
	}
	s, ok := p.readStat(strconv.Itoa(pid))
	if !ok {
		return procRaw{}, false
	}
	return procRaw{cpuSeconds: s.ticks / clockTicks, rss: s.rssPage * p.page, threads: s.threads, processes: 1}, true
}
