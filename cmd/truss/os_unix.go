//go:build unix

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// detach makes cmd survive the terminal that started it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// openBrowser asks the desktop to show url.
func openBrowser(url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	// #nosec G204 -- a fixed program, and a URL this command built itself.
	cmd := exec.Command(opener, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
