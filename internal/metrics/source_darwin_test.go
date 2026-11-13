package metrics

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func darwinSource(t *testing.T) machSource {
	t.Helper()
	src, why := newSource()
	if src == nil {
		t.Fatalf("no source: %s", why)
	}
	return src.(machSource)
}

func TestMachSource_Host(t *testing.T) {
	m := darwinSource(t)
	h, err := m.host()
	if err != nil {
		t.Fatal(err)
	}
	if len(h.cores) != runtime.NumCPU() {
		t.Errorf("cores = %d, NumCPU = %d", len(h.cores), runtime.NumCPU())
	}
	if h.all.total == 0 || h.all.busy > h.all.total {
		t.Errorf("all = %+v", h.all)
	}
	if h.memTotal == 0 || h.memUsed == 0 || h.memUsed > h.memTotal {
		t.Errorf("memory = %d of %d", h.memUsed, h.memTotal)
	}
	if len(h.load) != 3 {
		t.Errorf("load = %v", h.load)
	}
	// A second reading must not go backwards: the sampler takes deltas.
	time.Sleep(50 * time.Millisecond)
	h2, err := m.host()
	if err != nil || h2.all.total < h.all.total {
		t.Errorf("second reading %+v, %v after %+v", h2.all, err, h.all)
	}
}

func TestMachSource_ProcessCountsItsOwnCPU(t *testing.T) {
	m := darwinSource(t)
	before, ok := m.process(os.Getpid())
	if !ok || before.rss == 0 || before.threads == 0 || before.processes != 1 || before.tree {
		t.Fatalf("own process = %+v, %v", before, ok)
	}
	for end := time.Now().Add(200 * time.Millisecond); time.Now().Before(end); {
	}
	after, _ := m.process(os.Getpid())
	// 200 ms of spinning is at least 0.1 s of CPU, even on a loaded runner;
	// a wrong timebase is off by a factor of 40.
	if got := after.cpuSeconds - before.cpuSeconds; got < 0.1 || got > 2 {
		t.Fatalf("cpu went from %v to %v s over 200 ms of spinning", before.cpuSeconds, after.cpuSeconds)
	}
	if _, ok := m.process(0); ok {
		t.Error("pid 0 is reported")
	}
}

// TestMachSource_GroupSumsAStepsWholeProcessGroup starts a shell in its own
// process group, as the executor does for a step, with two children.
func TestMachSource_GroupSumsAStepsWholeProcessGroup(t *testing.T) {
	m := darwinSource(t)
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & sleep 30 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	pid := cmd.Process.Pid
	var g procRaw
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if g = m.groups([]int{pid})[pid]; g.processes == 3 {
			break
		}
	}
	if g.processes != 3 || !g.tree || g.rss == 0 || g.threads < 3 {
		t.Fatalf("group = %+v, want the shell and its two children", g)
	}
	if _, ok := m.groups([]int{pid + 1_000_000})[pid+1_000_000]; ok {
		t.Error("a group with no processes is reported")
	}
}

// TestMachSource_GroupKeepsTheCPUOfChildrenThatExited: a child that burned
// CPU and was waited for stays counted in the group.
func TestMachSource_GroupKeepsTheCPUOfChildrenThatExited(t *testing.T) {
	m := darwinSource(t)
	done := filepath.Join(t.TempDir(), "done")
	// The child spins, exits and is waited for; then the shell marks it and sleeps.
	spin := `/bin/sh -c 'i=0; while [ $i -lt 200000 ]; do i=$((i+1)); done'`
	cmd := exec.Command("/bin/sh", "-c", spin+` && : > "$DONE" && sleep 30`)
	cmd.Env = append(os.Environ(), "DONE="+done)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	for end := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(done); err == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("the spinning child did not finish")
		}
	}
	pid := cmd.Process.Pid
	// The loop is a fraction of a second of CPU. Too little means the child's
	// time was lost; far too much means a wrong time unit (Apple silicon's
	// timebase is 125/3).
	if g := m.groups([]int{pid})[pid]; g.cpuSeconds < 0.05 || g.cpuSeconds > 8 {
		t.Fatalf("group = %+v: want the spinning child's CPU, in seconds", g)
	}
}
