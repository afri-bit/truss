//go:build windows

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
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
	procNtResumeProcess    = ntdll.NewProc("NtResumeProcess")
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
	h, err := syscall.OpenProcess(processTerminate|processSetQuota|processSuspendResume, false, uint32(cmd.Process.Pid))
	if err != nil {
		return nil, fmt.Errorf("open process: %w", err)
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // the job owns the process from here
	job, _, callErr := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return nil, fmt.Errorf("create job object: %w", callErr)
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

// kill terminates every process in the job with exit code 1. Idempotent.
func (j *jobTree) kill() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.job != 0 {
		_, _, _ = procTerminateJobObject.Call(uintptr(j.job), 1)
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
