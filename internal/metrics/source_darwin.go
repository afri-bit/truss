package metrics

import "syscall"

func newSource() (source, string) {
	host := machHostSelf()
	if host == 0 {
		return nil, "the host port is not available"
	}
	numer, denom := machTimebase()
	if denom == 0 {
		return nil, "the Mach timebase is not available"
	}
	page, err := syscall.SysctlUint32("hw.pagesize")
	if err != nil || page == 0 {
		return nil, "the page size is not available"
	}
	return machSource{hostPort: host, task: taskSelf(), page: uint64(page), numer: numer, denom: denom}, ""
}

// machSource reads macOS through Mach host calls, libproc and sysctl. Like
// Linux it measures a step's whole process group (Tree is true).
type machSource struct {
	hostPort, task uint32 // ports, taken once: every lookup adds a reference
	page           uint64
	numer, denom   uint32 // Mach timebase: absolute units to nanoseconds
}

func (m machSource) host() (hostRaw, error) {
	var h hostRaw
	ticks, ok := processorTicks(m.hostPort, m.task)
	if !ok {
		return h, syscall.EIO
	}
	h.all, h.cores = machCPULoad(ticks)
	if s, err := syscall.Sysctl("hw.memsize"); err == nil {
		h.memTotal, _ = machUint64(s)
	}
	if vm, ok := vmStatistics(m.hostPort); ok {
		if used, ok := machMemoryUsed(vm, m.page); ok && used <= h.memTotal {
			h.memUsed = used
		}
	}
	if s, err := syscall.Sysctl("vm.loadavg"); err == nil {
		h.load = machLoad([]byte(s))
	}
	return h, nil
}

// read measures one process. With reaped, the CPU time of children it has
// waited for counts too, as on Linux, so a group total keeps the CPU of
// short-lived children (compilers, test binaries).
func (m machSource) read(pid int, reaped bool) (procRaw, bool) {
	b, ok := procTaskInfo(pid)
	if !ok {
		return procRaw{}, false
	}
	t, ok := machTaskInfo(b)
	if !ok {
		return procRaw{}, false
	}
	abs := t.cpuAbs
	if reaped {
		if ru, ok := procRusage(pid); ok {
			if r, ok := machReapedAbs(ru); ok {
				abs += r
			}
		}
	}
	return procRaw{cpuSeconds: absSeconds(abs, m.numer, m.denom), rss: t.rss, threads: t.threads, processes: 1}, true
}

// groups sums every process in each step's process group. The executor makes
// each step's shell the leader of its own group, so the group id is its pid.
func (m machSource) groups(pids []int) map[int]procRaw {
	out := map[int]procRaw{}
	for _, pgid := range pids {
		var g procRaw
		for _, pid := range groupPIDs(pgid) {
			p, ok := m.read(pid, true)
			if !ok {
				continue // ended between listing and reading
			}
			g.cpuSeconds += p.cpuSeconds
			g.rss += p.rss
			g.threads += p.threads
			g.processes++
		}
		if g.processes > 0 {
			g.tree = true
			out[pgid] = g
		}
	}
	return out
}

func (m machSource) process(pid int) (procRaw, bool) {
	if pid <= 0 {
		return procRaw{}, false
	}
	// Like Linux, the daemon's own reading leaves out its children: they are
	// steps, measured on their own.
	return m.read(pid, false)
}
