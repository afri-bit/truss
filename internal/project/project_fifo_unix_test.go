//go:build unix

package project

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A named pipe at truss.yaml must be refused without opening it: opening a
// pipe with no writer blocks forever.
func TestOpen_RefusesANamedPipeWithoutHanging(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "truss.yaml"), 0o600); err != nil {
		t.Skipf("cannot make a named pipe here: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Open(root)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Open accepted a named pipe")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Open blocked on a named pipe")
	}
}

func TestReadRegular_RefusesANamedPipeWithoutHanging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("cannot make a named pipe here: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readRegular(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("readRegular accepted a named pipe")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readRegular blocked on a named pipe")
	}
}
