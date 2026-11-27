package runs

import "sync"

// MaxRunLog bounds the step output a daemon run keeps in its log (spec
// 028), so a step that prints without end cannot fill the disk. TRUSS's own
// lines, the summary included, are kept beyond it.
const MaxRunLog = 64 << 20

// outputBudget counts the step output a run may still log. Steps of one run
// may print at the same time.
type outputBudget struct {
	mu   sync.Mutex
	left int64
	cut  bool
}

// take returns how many of n bytes may be logged, and whether this is the
// moment to say that the rest is dropped (once).
func (b *outputBudget) take(n int) (keep int, notice bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cut {
		return 0, false
	}
	if int64(n) <= b.left {
		b.left -= int64(n)
		return n, false
	}
	keep = int(b.left)
	b.left, b.cut = 0, true
	return keep, true
}
