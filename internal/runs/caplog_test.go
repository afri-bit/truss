package runs

import "testing"

func TestOutputBudget_KeepsUpToTheLimitAndSaysSoOnce(t *testing.T) {
	b := outputBudget{left: 10}
	for _, c := range []struct {
		n, keep int
		notice  bool
	}{
		{5, 5, false}, {3, 3, false}, {6, 2, true}, {4, 0, false},
	} {
		if keep, notice := b.take(c.n); keep != c.keep || notice != c.notice {
			t.Fatalf("take(%d) = %d, %v; want %d, %v", c.n, keep, notice, c.keep, c.notice)
		}
	}
}

// Empty lines are cheap to print but not to log (security review, spec 028).
func TestLogCost_CountsWhatTheLogWrites(t *testing.T) {
	if got := logCost("build/pkg", []byte("\n\n\n")); got != 3+3*(28+9) {
		t.Fatalf("logCost = %d", got)
	}
}
