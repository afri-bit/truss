//go:build linux || darwin

package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A named pipe in place of the log must be refused, not opened: opening it
// would block for ever.
func TestDaemonLogs_ANamedPipeIsRefused(t *testing.T) {
	s := newSession(t, false)
	if err := syscall.Mkfifo(filepath.Join(s.home, "daemon.log"), 0o600); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if code, _, errOut := s.truss("daemon", "logs"); code != 1 || !strings.Contains(errOut, "not a regular file") {
			t.Errorf("exit %d, stderr %q", code, errOut)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("daemon logs blocked on a named pipe")
	}
}
