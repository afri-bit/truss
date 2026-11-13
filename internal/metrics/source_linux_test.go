package metrics

import "testing"

func fixture() procfs { return procfs{root: "testdata/proc", page: 4096} }

func TestProcfs_Host(t *testing.T) {
	h, err := fixture().host()
	if err != nil {
		t.Fatal(err)
	}
	// cpu: user 1000, system 500, idle 8000, iowait 500 -> busy 1500 of 10000
	if h.all.busy != 1500 || h.all.total != 10000 {
		t.Errorf("all = %+v", h.all)
	}
	if len(h.cores) != 2 || h.cores[0].busy != 900 || h.cores[1].total != 5000 {
		t.Errorf("cores = %+v", h.cores)
	}
	if h.memTotal != 16384000*1024 || h.memUsed != (16384000-8192000)*1024 {
		t.Errorf("memory = %d of %d", h.memUsed, h.memTotal)
	}
	if len(h.load) != 3 || h.load[0] != 0.52 || h.load[2] != 0.59 {
		t.Errorf("load = %v", h.load)
	}
}

func TestProcfs_GroupsSumAStepsWholeProcessGroup(t *testing.T) {
	g := fixture().groups([]int{100, 555})
	got, ok := g[100]
	if !ok {
		t.Fatalf("group 100 missing: %+v", g)
	}
	// pid 100 (40 ticks, 250 pages) + pid 101 "my (weird) tool" (60 ticks, 1000 pages); pid 102 is another group.
	if got.cpuSeconds != 1.0 || got.rss != 1250*4096 || got.processes != 2 || got.threads != 4 || !got.tree {
		t.Errorf("group 100 = %+v", got)
	}
	if _, ok := g[555]; ok {
		t.Error("a group with no processes is reported")
	}
}

func TestProcfs_Process(t *testing.T) {
	p, ok := fixture().process(200)
	if !ok || p.cpuSeconds != 0.6 || p.rss != 2000*4096 || p.threads != 12 || p.processes != 1 || p.tree {
		t.Fatalf("process 200 = %+v, %v", p, ok)
	}
	if _, ok := fixture().process(999); ok {
		t.Error("a missing process is reported")
	}
}
