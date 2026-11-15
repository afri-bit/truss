package metrics

// Calls into libSystem without cgo, the way golang.org/x/sys/unix makes them:
// each function is imported dynamically, reached through an assembly
// trampoline (libsystem_darwin.s), and called with the runtime's
// syscall.syscall, which the runtime keeps for this use (go.dev/issue/67401).
// See ADR 0013.

import (
	"syscall"
	"unsafe"
)

//go:linkname syscallSyscall syscall.syscall
func syscallSyscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:linkname syscallSyscall6 syscall.syscall6
func syscallSyscall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

// Trampoline addresses, set by libsystem_darwin.s. The names follow the
// assembly symbols.
var (
	libcMachHostSelfTrampolineAddr      uintptr
	libcTaskSelfTrapTrampolineAddr      uintptr
	libcMachTimebaseInfoTrampolineAddr  uintptr
	libcHostPageSizeTrampolineAddr      uintptr
	libcHostProcessorInfoTrampolineAddr uintptr
	libcHostStatistics64TrampolineAddr  uintptr
	libcVMDeallocateTrampolineAddr      uintptr
	libcProcPidinfoTrampolineAddr       uintptr
	libcProcPidRusageTrampolineAddr     uintptr
	libcProcListpidsTrampolineAddr      uintptr
)

//go:cgo_import_dynamic libc_mach_host_self mach_host_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_task_self_trap task_self_trap "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_mach_timebase_info mach_timebase_info "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_host_page_size host_page_size "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_host_processor_info host_processor_info "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_host_statistics64 host_statistics64 "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_vm_deallocate vm_deallocate "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_proc_pidinfo proc_pidinfo "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_proc_pid_rusage proc_pid_rusage "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_proc_listpids proc_listpids "/usr/lib/libSystem.B.dylib"

// Constants from the macOS SDK headers.
const (
	processorCPULoadInfo = 2 // PROCESSOR_CPU_LOAD_INFO, mach/processor_info.h
	hostVMInfo64         = 4 // HOST_VM_INFO64, mach/host_info.h
	procPIDTaskInfo      = 4 // PROC_PIDTASKINFO, sys/proc_info.h
	procPGRPOnly         = 2 // PROC_PGRP_ONLY, libproc.h
	rusageInfoV2         = 2 // RUSAGE_INFO_V2, sys/resource.h
	kernSuccess          = 0 // KERN_SUCCESS
	integerSize          = 4 // sizeof(integer_t)
	pidSize              = 4 // sizeof(pid_t)
	maxGroupPIDs         = 1 << 16
)

// machHostSelf returns a send right to the host port. Each call adds a
// reference, so callers keep the one they get.
func machHostSelf() uint32 {
	r, _, _ := syscallSyscall(libcMachHostSelfTrampolineAddr, 0, 0, 0)
	return uint32(r)
}

// taskSelf returns a send right to this task's port, like mach_task_self().
func taskSelf() uint32 {
	r, _, _ := syscallSyscall(libcTaskSelfTrapTrampolineAddr, 0, 0, 0)
	return uint32(r)
}

func machTimebase() (numer, denom uint32) {
	var tb struct{ numer, denom uint32 }
	if r, _, _ := syscallSyscall(libcMachTimebaseInfoTrampolineAddr, uintptr(unsafe.Pointer(&tb)), 0, 0); int32(r) != kernSuccess {
		return 0, 0
	}
	return tb.numer, tb.denom
}

// hostPageSize is the kernel's page size, the unit of host_statistics64's
// page counts. It differs from hw.pagesize, the caller's page size, when an
// amd64 build runs under Rosetta on Apple silicon (4 KiB against 16 KiB).
func hostPageSize(host uint32) uint64 {
	var size uintptr
	if r, _, _ := syscallSyscall(libcHostPageSizeTrampolineAddr, uintptr(host), uintptr(unsafe.Pointer(&size)), 0); int32(r) != kernSuccess {
		return 0
	}
	return uint64(size)
}

// processorTicks returns PROCESSOR_CPU_LOAD_INFO for every processor, copied
// out of the buffer the kernel allocates, which it then frees.
func processorTicks(host, task uint32) ([]uint32, bool) {
	var count, infoCount uint32
	// The kernel stores the address of a buffer it allocated, outside the Go heap.
	var info *uint32
	r, _, _ := syscallSyscall6(libcHostProcessorInfoTrampolineAddr, uintptr(host), processorCPULoadInfo,
		uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&infoCount)), 0)
	if int32(r) != kernSuccess || info == nil {
		return nil, false
	}
	// Copy the buffer before giving it back.
	ticks := append([]uint32(nil), unsafe.Slice(info, infoCount)...)
	_, _, _ = syscallSyscall(libcVMDeallocateTrampolineAddr, uintptr(task), uintptr(unsafe.Pointer(info)), uintptr(infoCount)*integerSize)
	return ticks, true
}

func vmStatistics(host uint32) ([]byte, bool) {
	buf := make([]byte, vmStatistics64Size)
	count := uint32(vmStatistics64Size / integerSize)
	r, _, _ := syscallSyscall6(libcHostStatistics64TrampolineAddr, uintptr(host), hostVMInfo64,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&count)), 0, 0)
	return buf, int32(r) == kernSuccess
}

func procTaskInfo(pid int) ([]byte, bool) {
	buf := make([]byte, procTaskInfoSize)
	r, _, _ := syscallSyscall6(libcProcPidinfoTrampolineAddr, uintptr(pid), procPIDTaskInfo, 0,
		uintptr(unsafe.Pointer(&buf[0])), procTaskInfoSize, 0)
	return buf, int(int32(r)) == procTaskInfoSize
}

func procRusage(pid int) ([]byte, bool) {
	buf := make([]byte, rusageInfoV2Size)
	r, _, _ := syscallSyscall(libcProcPidRusageTrampolineAddr, uintptr(pid), rusageInfoV2, uintptr(unsafe.Pointer(&buf[0])))
	return buf, int32(r) == 0
}

// groupPIDs lists the processes whose process group is pgid.
func groupPIDs(pgid int) []int {
	for n := 256; n <= maxGroupPIDs; n *= 4 {
		buf := make([]int32, n)
		r, _, _ := syscallSyscall6(libcProcListpidsTrampolineAddr, procPGRPOnly, uintptr(pgid),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(n*pidSize), 0, 0)
		got := int(int32(r)) / pidSize
		if got <= 0 {
			return nil
		}
		if got < n { // a full buffer may have been cut short
			out := make([]int, 0, got)
			for _, p := range buf[:got] {
				if p > 0 {
					out = append(out, int(p))
				}
			}
			return out
		}
	}
	return nil
}
