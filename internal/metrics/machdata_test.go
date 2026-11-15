package metrics

import (
	"encoding/binary"
	"testing"
)

func TestMachCPULoad_SumsCoresAndCountsNiceAsBusy(t *testing.T) {
	// Two processors: user, system, idle, nice.
	all, cores := machCPULoad([]uint32{100, 50, 800, 50, 10, 10, 980, 0})
	if len(cores) != 2 || cores[0] != (cpuTimes{busy: 200, total: 1000}) || cores[1] != (cpuTimes{busy: 20, total: 1000}) {
		t.Fatalf("cores = %+v", cores)
	}
	if all != (cpuTimes{busy: 220, total: 2000}) {
		t.Fatalf("all = %+v", all)
	}
	if all, cores := machCPULoad([]uint32{1, 2, 3}); len(cores) != 0 || all.total != 0 {
		t.Fatalf("a partial processor is counted: %+v %+v", all, cores)
	}
}

func TestMachMemoryUsed_IsAppPlusWiredPlusCompressed(t *testing.T) {
	b := make([]byte, vmStatistics64Size)
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
	put(0, 9999)  // free: not used
	put(4, 9999)  // active: not counted directly
	put(12, 100)  // wired
	put(88, 30)   // purgeable
	put(128, 20)  // compressor
	put(136, 500) // external (file cache): not used
	put(140, 230) // internal
	got, ok := machMemoryUsed(b, 16384)
	if want := uint64(200+100+20) * 16384; !ok || got != want {
		t.Fatalf("used = %d, %v; want %d", got, ok, want)
	}
	if _, ok := machMemoryUsed(b[:100], 16384); ok {
		t.Fatal("a short buffer is accepted")
	}
}

func TestMachLoad_ScalesAndToleratesADroppedZeroByte(t *testing.T) {
	b := make([]byte, 24)
	binary.LittleEndian.PutUint32(b[0:], 1024) // 0.5
	binary.LittleEndian.PutUint32(b[4:], 2048) // 1
	binary.LittleEndian.PutUint32(b[8:], 3072) // 1.5
	binary.LittleEndian.PutUint64(b[16:], 2048)
	// syscall.Sysctl drops one trailing zero byte.
	got := machLoad(b[:23])
	if len(got) != 3 || got[0] != 0.5 || got[1] != 1 || got[2] != 1.5 {
		t.Fatalf("load = %v", got)
	}
	if machLoad(b[:8]) != nil {
		t.Fatal("a short buffer gives figures")
	}
}

func TestMachUint64_RestoresADroppedZeroByte(t *testing.T) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, 16<<30) // 16 GiB: the top bytes are zero
	if got, ok := machUint64(string(b[:7])); !ok || got != 16<<30 {
		t.Fatalf("got %d, %v", got, ok)
	}
	if _, ok := machUint64(""); ok {
		t.Fatal("an empty value is accepted")
	}
}

func TestMachTaskInfo_ReadsResidentCPUAndThreads(t *testing.T) {
	b := make([]byte, procTaskInfoSize)
	le := binary.LittleEndian
	le.PutUint64(b[0:], 1<<40) // virtual size: not used
	le.PutUint64(b[8:], 5<<20)
	le.PutUint64(b[16:], 3000)
	le.PutUint64(b[24:], 1000)
	le.PutUint32(b[84:], 7)
	got, ok := machTaskInfo(b)
	if !ok || got != (machTask{rss: 5 << 20, cpuAbs: 4000, threads: 7}) {
		t.Fatalf("task = %+v, %v", got, ok)
	}
	if _, ok := machTaskInfo(b[:95]); ok {
		t.Fatal("a short buffer is accepted")
	}
}

func TestMachReapedAbs_AddsChildUserAndSystemTime(t *testing.T) {
	b := make([]byte, rusageInfoV2Size)
	binary.LittleEndian.PutUint64(b[16:], 999) // own user time: not reaped
	binary.LittleEndian.PutUint64(b[96:], 70)
	binary.LittleEndian.PutUint64(b[104:], 30)
	if got, ok := machReapedAbs(b); !ok || got != 100 {
		t.Fatalf("reaped = %d, %v", got, ok)
	}
}

func TestAbsSeconds_UsesTheTimebase(t *testing.T) {
	if got := absSeconds(2e9, 1, 1); got != 2 {
		t.Errorf("intel: %v", got)
	}
	if got := absSeconds(24e6, 125, 3); got != 1 { // Apple silicon: 24 MHz ticks
		t.Errorf("apple silicon: %v", got)
	}
	if got := absSeconds(5, 1, 0); got != 0 {
		t.Errorf("zero denominator: %v", got)
	}
}
