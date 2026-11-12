//go:build unix

package daemon

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

// A registered file replaced by a FIFO must not hang the daemon.
func TestDaemon_ARegisteredFileSwappedForAFIFODoesNotHangTheDaemon(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	path := writeDefinition(t, t.TempDir(), "p.yaml", scheduled("skip"))
	if _, err := c.Register(ctx, api.RegisterRequest{Path: path, Name: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = c.Pipelines(ctx)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("listing pipelines hung on a FIFO")
	}
}
