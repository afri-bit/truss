//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// openNoBlock is zero here: opening a file never blocks on Windows.
const openNoBlock = 0

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detach makes cmd survive the console that started it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}

// openBrowser asks the desktop to show url.
func openBrowser(url string) error {
	// #nosec G204 -- a fixed program, and a URL this command built itself.
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
