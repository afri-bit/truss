//go:build windows

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	createSuspended      = 0x4
	processTerminate     = 0x1
	processSetQuota      = 0x100
	processSuspendResume = 0x800
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	ntdll                  = syscall.NewLazyDLL("ntdll.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject = kernel32.NewProc("TerminateJobObject")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procNtResumeProcess    = ntdll.NewProc("NtResumeProcess")
)

// jobObjectExtendedLimitInformation is JOBOBJECT_EXTENDED_LIMIT_INFORMATION;
// the field order and sizes follow the C struct on 64-bit Windows.
type jobObjectExtendedLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	IoCounters              [6]uint64
	ProcessMemoryLimit      uintptr
	JobMemoryLimit          uintptr
	PeakProcessMemoryUsed   uintptr
	PeakJobMemoryUsed       uintptr
}

// The struct must have the C size on the architectures TRUSS ships for
// Windows (amd64, arm64): a build for another one fails here instead of at
// every step's start.
var (
	_ [144 - unsafe.Sizeof(jobObjectExtendedLimitInformation{})]byte
	_ [unsafe.Sizeof(jobObjectExtendedLimitInformation{}) - 144]byte
)

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x2000
)

// jobTree is a job object that holds the shell and its descendants.
// BREAKAWAY_OK is not set, so no descendant can leave the job.
type jobTree struct {
	mu  sync.Mutex
	job syscall.Handle
}

// prepare starts the shell suspended so no user code runs before it is in the job.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createSuspended}
}

// attach puts the suspended shell into a new job object and resumes it.
func attach(cmd *exec.Cmd) (tree, error) {
	h, err := syscall.OpenProcess(processTerminate|processSetQuota|processSuspendResume, false, uint32(cmd.Process.Pid)) //nolint:gosec // G115: a Windows pid is a DWORD
	if err != nil {
		return nil, fmt.Errorf("open process: %w", err)
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // the job owns the process from here
	job, _, callErr := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create job object: %w", callErr)
	}
	// When the last handle to the job closes, every process in it ends: if
	// TRUSS itself dies, its steps do not outlive it (spec 028).
	info := jobObjectExtendedLimitInformation{LimitFlags: jobObjectLimitKillOnJobClose}
	if r, _, callErr := procSetInformationJob.Call(job, jobObjectExtendedLimitInformationClass,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); r == 0 { // #nosec G103 -- a Win32 call needs the struct's address
		_ = syscall.CloseHandle(syscall.Handle(job))
		return nil, fmt.Errorf("set job object limits: %w", callErr)
	}
	if r, _, callErr := procAssignProcessToJob.Call(job, uintptr(h)); r == 0 {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return nil, fmt.Errorf("assign process to job object: %w", callErr)
	}
	if status, _, _ := procNtResumeProcess.Call(uintptr(h)); status != 0 {
		_ = syscall.CloseHandle(syscall.Handle(job)) // closing does not kill; Start kills the shell
		return nil, fmt.Errorf("resume process: status 0x%x", status)
	}
	return &jobTree{job: syscall.Handle(job)}, nil
}

// stoppedExitCode is the exit code TRUSS gives every process it ends through
// the job object. It is distinctive (0x5452, "TR") so a shell that was ended by
// TRUSS can be told apart from one that exited by itself.
const stoppedExitCode = 0x5452

// terminate reports false: a job object has no polite stop that reaches
// every process, so a stopped step on Windows is ended at once (spec 028).
func (j *jobTree) terminate() bool { return false }

// waitGone is not needed on Windows: terminate reports false, so the job is
// killed at once.
func (j *jobTree) waitGone(time.Time, <-chan struct{}) {}

// kill terminates every process in the job with stoppedExitCode. Idempotent.
func (j *jobTree) kill() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.job != 0 {
		_, _, _ = procTerminateJobObject.Call(uintptr(j.job), stoppedExitCode)
	}
}

// afterExit terminates what is left of the job.
func (j *jobTree) afterExit() { j.kill() }

// release closes the job handle.
func (j *jobTree) release() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.job != 0 {
		_ = syscall.CloseHandle(j.job)
		j.job = 0
	}
}

// signalName: Windows has no POSIX signals, so a missing exit code is unknown.
func signalName(*os.ProcessState) string { return "unknown" }
