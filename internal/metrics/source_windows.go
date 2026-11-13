package metrics

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	ntdll                     = syscall.NewLazyDLL("ntdll.dll")
	procGetSystemTimes        = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx  = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetProcessMemoryInfo  = kernel32.NewProc("K32GetProcessMemoryInfo")
	procNtQuerySystemInfo     = ntdll.NewProc("NtQuerySystemInformation")
	processQueryLimitedInfo   = uint32(0x1000)
	systemProcessorPerfInfo   = uintptr(8)
	processorPerfInfoByteSize = 48
)

func newSource() (source, string) {
	if err := procGetSystemTimes.Find(); err != nil {
		return nil, "GetSystemTimes is not available"
	}
	return winSource{}, ""
}

// winSource reads Windows through kernel32 and ntdll. It measures a step's
// shell process only: the processes it starts are not added (Tree is false).
type winSource struct{}

func filetime(ft syscall.Filetime) float64 {
	return float64(uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)) // 100 ns units
}

func (winSource) host() (hostRaw, error) {
	var h hostRaw
	var idle, kernel, user syscall.Filetime
	if r, _, err := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return h, err
	}
	// Kernel time includes idle time.
	total := filetime(kernel) + filetime(user)
	h.all = cpuTimes{busy: total - filetime(idle), total: total}

	if err := procNtQuerySystemInfo.Find(); err == nil {
		buf := make([]byte, 256*processorPerfInfoByteSize)
		var got uint32
		status, _, _ := procNtQuerySystemInfo.Call(systemProcessorPerfInfo, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&got)))
		if status == 0 {
			for off := 0; off+processorPerfInfoByteSize <= int(got); off += processorPerfInfoByteSize {
				// IdleTime, KernelTime, UserTime: three LARGE_INTEGERs at the start.
				idle := float64(*(*int64)(unsafe.Pointer(&buf[off])))
				kern := float64(*(*int64)(unsafe.Pointer(&buf[off+8])))
				usr := float64(*(*int64)(unsafe.Pointer(&buf[off+16])))
				h.cores = append(h.cores, cpuTimes{busy: kern + usr - idle, total: kern + usr})
			}
		}
	}

	// MEMORYSTATUSEX
	var mem struct {
		length               uint32
		memoryLoad           uint32
		totalPhys, availPhys uint64
		totalPage, availPage uint64
		totalVirt, availVirt uint64
		availExtendedVirtual uint64
	}
	mem.length = uint32(unsafe.Sizeof(mem))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
		h.memTotal = mem.totalPhys
		h.memUsed = mem.totalPhys - mem.availPhys
	}
	return h, nil
}

func (s winSource) groups(pids []int) map[int]procRaw {
	out := map[int]procRaw{}
	for _, pid := range pids {
		if r, ok := s.process(pid); ok {
			out[pid] = r
		}
	}
	return out
}

func (winSource) process(pid int) (procRaw, bool) {
	if pid <= 0 {
		return procRaw{}, false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInfo, false, uint32(pid)) //nolint:gosec // G115: a Windows pid is a DWORD
	if err != nil {
		return procRaw{}, false
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // nothing to do if closing fails
	var create, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &create, &exit, &kernel, &user); err != nil {
		return procRaw{}, false
	}
	r := procRaw{cpuSeconds: (filetime(kernel) + filetime(user)) / 1e7, processes: 1}
	// PROCESS_MEMORY_COUNTERS
	var pmc struct {
		cb                         uint32
		pageFaultCount             uint32
		peakWorkingSetSize         uintptr
		workingSetSize             uintptr
		quotaPeakPagedPoolUsage    uintptr
		quotaPagedPoolUsage        uintptr
		quotaPeakNonPagedPoolUsage uintptr
		quotaNonPagedPoolUsage     uintptr
		pagefileUsage              uintptr
		peakPagefileUsage          uintptr
	}
	pmc.cb = uint32(unsafe.Sizeof(pmc))
	if ok, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.cb)); ok != 0 {
		r.rss = uint64(pmc.workingSetSize)
	}
	return r, true
}
