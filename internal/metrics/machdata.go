package metrics

import "encoding/binary"

// Decoders for the structures macOS returns. They are plain byte parsers with
// no build constraint, so their tests run on every system; only the macOS
// source calls them. macOS runs on little-endian amd64 and arm64 only.

// The CPU states of PROCESSOR_CPU_LOAD_INFO, in this order per processor.
const (
	machCPUUser = iota
	machCPUSystem
	machCPUIdle
	machCPUNice
	machCPUStates
)

// machCPULoad turns per-processor tick counters (machCPUStates per
// processor) into the machine's total and one entry per logical core.
func machCPULoad(ticks []uint32) (all cpuTimes, cores []cpuTimes) {
	for i := 0; i+machCPUStates <= len(ticks); i += machCPUStates {
		c := [machCPUStates]uint32(ticks[i : i+machCPUStates])
		busy := float64(c[machCPUUser]) + float64(c[machCPUSystem]) + float64(c[machCPUNice])
		t := cpuTimes{busy: busy, total: busy + float64(c[machCPUIdle])}
		cores = append(cores, t)
		all.busy += t.busy
		all.total += t.total
	}
	return all, cores
}

// vmStatistics64Size is the byte size of vm_statistics64 that macOS fills in
// for HOST_VM_INFO64.
const vmStatistics64Size = 152

// machMemoryUsed is the memory Activity Monitor calls "Memory Used": app
// memory (anonymous pages that cannot be purged), wired memory and the
// compressor's pages. b is a vm_statistics64; page is the page size in bytes.
func machMemoryUsed(b []byte, page uint64) (uint64, bool) {
	if len(b) < vmStatistics64Size || page == 0 {
		return 0, false
	}
	u32 := func(off int) uint64 { return uint64(binary.LittleEndian.Uint32(b[off:])) }
	wired := u32(12)
	purgeable := u32(88)
	compressor := u32(128)
	internal := u32(140)
	app := uint64(0)
	if internal > purgeable {
		app = internal - purgeable
	}
	return (app + wired + compressor) * page, true
}

// machLoad turns a struct loadavg (three fixed-point values and their scale)
// into the 1, 5 and 15 minute load averages. b may lack trailing zero bytes:
// syscall.Sysctl drops the last one.
func machLoad(b []byte) []float64 {
	var raw [24]byte
	if len(b) > len(raw) || len(b) < 12 {
		return nil
	}
	copy(raw[:], b)
	scale := float64(binary.LittleEndian.Uint64(raw[16:]))
	if scale == 0 {
		return nil
	}
	out := make([]float64, 3)
	for i := range out {
		out[i] = float64(binary.LittleEndian.Uint32(raw[4*i:])) / scale
	}
	return out
}

// machUint64 reads a 64-bit sysctl value as syscall.Sysctl returns it, which
// may have dropped one trailing zero byte.
func machUint64(s string) (uint64, bool) {
	if len(s) == 0 || len(s) > 8 {
		return 0, false
	}
	var raw [8]byte
	copy(raw[:], s)
	return binary.LittleEndian.Uint64(raw[:]), true
}

// procTaskInfoSize is the byte size of struct proc_taskinfo
// (proc_pidinfo with PROC_PIDTASKINFO).
const procTaskInfoSize = 96

// machTask is the part of proc_taskinfo that metrics uses. CPU times are in
// Mach absolute time units.
type machTask struct {
	rss     uint64
	cpuAbs  uint64
	threads int
}

func machTaskInfo(b []byte) (machTask, bool) {
	if len(b) < procTaskInfoSize {
		return machTask{}, false
	}
	le := binary.LittleEndian
	return machTask{
		rss:     le.Uint64(b[8:]),
		cpuAbs:  le.Uint64(b[16:]) + le.Uint64(b[24:]), // pti_total_user + pti_total_system
		threads: int(le.Uint32(b[84:])),                // pti_threadnum, never negative
	}, true
}

// rusageInfoV2Size is the byte size of struct rusage_info_v2.
const rusageInfoV2Size = 160

// machReapedAbs is the CPU time, in Mach absolute time units, of the children
// a process has waited for (ri_child_user_time + ri_child_system_time).
func machReapedAbs(b []byte) (uint64, bool) {
	if len(b) < rusageInfoV2Size {
		return 0, false
	}
	le := binary.LittleEndian
	return le.Uint64(b[96:]) + le.Uint64(b[104:]), true
}

// absSeconds converts Mach absolute time units to seconds with the machine's
// timebase (1/1 on Intel, 125/3 on Apple silicon).
func absSeconds(abs uint64, numer, denom uint32) float64 {
	if denom == 0 {
		return 0
	}
	return float64(abs) * float64(numer) / float64(denom) / 1e9
}
