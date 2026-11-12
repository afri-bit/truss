//go:build linux || darwin

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a terminal.
func isTerminal(f *os.File) bool {
	var t syscall.Termios
	return ioctl(f.Fd(), getTermios, &t) == nil
}

// disableEcho turns off echo on the terminal f and returns a function that
// restores the previous settings.
func disableEcho(f *os.File) (func(), error) {
	var old syscall.Termios
	if err := ioctl(f.Fd(), getTermios, &old); err != nil {
		return nil, err
	}
	quiet := old
	quiet.Lflag &^= syscall.ECHO
	if err := ioctl(f.Fd(), setTermios, &quiet); err != nil {
		return nil, err
	}
	return func() { _ = ioctl(f.Fd(), setTermios, &old) }, nil
}

func ioctl(fd, req uintptr, t *syscall.Termios) error {
	// #nosec G103 -- the pointer is to a local Termios, passed to the kernel for one call.
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t))); e != 0 {
		return e
	}
	return nil
}
