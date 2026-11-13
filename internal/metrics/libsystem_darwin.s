// Trampolines to the libSystem functions imported in libsystem_darwin.go.
// The same text assembles for darwin/amd64 and darwin/arm64.

#include "textflag.h"

TEXT libc_mach_host_self_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_mach_host_self(SB)
GLOBL	·libcMachHostSelfTrampolineAddr(SB), RODATA, $8
DATA	·libcMachHostSelfTrampolineAddr(SB)/8, $libc_mach_host_self_trampoline<>(SB)

TEXT libc_task_self_trap_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_task_self_trap(SB)
GLOBL	·libcTaskSelfTrapTrampolineAddr(SB), RODATA, $8
DATA	·libcTaskSelfTrapTrampolineAddr(SB)/8, $libc_task_self_trap_trampoline<>(SB)

TEXT libc_mach_timebase_info_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_mach_timebase_info(SB)
GLOBL	·libcMachTimebaseInfoTrampolineAddr(SB), RODATA, $8
DATA	·libcMachTimebaseInfoTrampolineAddr(SB)/8, $libc_mach_timebase_info_trampoline<>(SB)

TEXT libc_host_page_size_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_host_page_size(SB)
GLOBL	·libcHostPageSizeTrampolineAddr(SB), RODATA, $8
DATA	·libcHostPageSizeTrampolineAddr(SB)/8, $libc_host_page_size_trampoline<>(SB)

TEXT libc_host_processor_info_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_host_processor_info(SB)
GLOBL	·libcHostProcessorInfoTrampolineAddr(SB), RODATA, $8
DATA	·libcHostProcessorInfoTrampolineAddr(SB)/8, $libc_host_processor_info_trampoline<>(SB)

TEXT libc_host_statistics64_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_host_statistics64(SB)
GLOBL	·libcHostStatistics64TrampolineAddr(SB), RODATA, $8
DATA	·libcHostStatistics64TrampolineAddr(SB)/8, $libc_host_statistics64_trampoline<>(SB)

TEXT libc_vm_deallocate_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_vm_deallocate(SB)
GLOBL	·libcVMDeallocateTrampolineAddr(SB), RODATA, $8
DATA	·libcVMDeallocateTrampolineAddr(SB)/8, $libc_vm_deallocate_trampoline<>(SB)

TEXT libc_proc_pidinfo_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_pidinfo(SB)
GLOBL	·libcProcPidinfoTrampolineAddr(SB), RODATA, $8
DATA	·libcProcPidinfoTrampolineAddr(SB)/8, $libc_proc_pidinfo_trampoline<>(SB)

TEXT libc_proc_pid_rusage_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_pid_rusage(SB)
GLOBL	·libcProcPidRusageTrampolineAddr(SB), RODATA, $8
DATA	·libcProcPidRusageTrampolineAddr(SB)/8, $libc_proc_pid_rusage_trampoline<>(SB)

TEXT libc_proc_listpids_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_proc_listpids(SB)
GLOBL	·libcProcListpidsTrampolineAddr(SB), RODATA, $8
DATA	·libcProcListpidsTrampolineAddr(SB)/8, $libc_proc_listpids_trampoline<>(SB)
