//go:build windows

package main

import (
	"os"
	"syscall"
)

const enableEchoInput = 0x0004

// isTerminal reports whether f is a console.
func isTerminal(f *os.File) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(f.Fd()), &mode) == nil
}

// disableEcho turns off echo on the console f and returns a function that
// restores the previous mode.
func disableEcho(f *os.File) (func(), error) {
	h := syscall.Handle(f.Fd())
	var old uint32
	if err := syscall.GetConsoleMode(h, &old); err != nil {
		return nil, err
	}
	if err := setConsoleMode(h, old&^enableEchoInput); err != nil {
		return nil, err
	}
	return func() { _ = setConsoleMode(h, old) }, nil
}

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procSetConsole = kernel32.NewProc("SetConsoleMode")
)

func setConsoleMode(h syscall.Handle, mode uint32) error {
	r, _, err := procSetConsole.Call(uintptr(h), uintptr(mode))
	if r == 0 {
		return err
	}
	return nil
}
