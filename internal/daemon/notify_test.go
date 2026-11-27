package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/afri-bit/truss/internal/api"
	"github.com/afri-bit/truss/internal/home"
)

// Spec 030: a failed run notifies through the flow's notify run command; a
// recovery does too when asked; a success alone does not.
func TestNotify_TheDaemonNotifiesAsTheFlowAsks(t *testing.T) {
	ctx := context.Background()
	_, c := boot(t, home.Dir(t.TempDir()), nil)
	dir := t.TempDir()
	log := filepath.Join(dir, "notified.txt")
	tell := "echo $TRUSS_NOTIFY_STATUS >> \"" + log + "\""
	fail := "test -e \"" + filepath.Join(dir, "ok") + "\""
	if runtime.GOOS == "windows" {
		tell = "echo %TRUSS_NOTIFY_STATUS%>> \"" + log + "\""
		fail = "if not exist \"" + filepath.Join(dir, "ok") + "\" exit 1"
	}
	flow := writeDefinition(t, dir, "n.yaml", "version: \"1\"\nflow: {name: n}\nnotify:\n  on: [failure, recovered]\n  run: '"+
		strings.ReplaceAll(tell, "'", "''")+"'\nsteps:\n  - id: a\n    run: '"+strings.ReplaceAll(fail, "'", "''")+"'\n")
	if _, err := c.Register(ctx, api.RegisterRequest{Path: flow}); err != nil {
		t.Fatal(err)
	}
	waitLines := func(want int) []string {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			data, _ := os.ReadFile(log)
			lines := strings.Fields(string(data))
			if len(lines) >= want || time.Now().After(deadline) {
				return lines
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	if _, err := c.Trigger(ctx, "n"); err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "n", 1)
	if lines := waitLines(1); len(lines) != 1 || lines[0] != "failed" {
		t.Fatalf("after a failure: %v", lines)
	}
	writeDefinition(t, dir, "ok", "")
	if _, err := c.Trigger(ctx, "n"); err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "n", 2)
	if lines := waitLines(2); len(lines) != 2 || lines[1] != "succeeded" {
		t.Fatalf("after a recovery: %v", lines)
	}
	if _, err := c.Trigger(ctx, "n"); err != nil {
		t.Fatal(err)
	}
	awaitRuns(t, c, "n", 3)
	time.Sleep(500 * time.Millisecond)
	if lines := waitLines(3); len(lines) != 2 {
		t.Fatalf("a plain success notified: %v", lines)
	}
}
