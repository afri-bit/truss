//go:build unix

package pipeline

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Opening a named pipe with no writer blocks forever, so ParseFile must look
// at the file kind first.
func TestParseFile_RefusesANamedPipeWithoutHanging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.yaml")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot make a named pipe here: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := ParseFile(context.Background(), path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ParseFile accepted a named pipe")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ParseFile blocked on a named pipe")
	}
}
