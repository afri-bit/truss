package runs

import (
	"strings"
	"testing"
)

func TestCappedLog_StopsAtTheLimitAndSaysSo(t *testing.T) {
	var b strings.Builder
	c := &cappedLog{w: &b, left: 10}
	for _, s := range []string{"12345", "678", "abcdef", "more"} {
		if n, err := c.Write([]byte(s)); err != nil || n != len(s) {
			t.Fatalf("Write(%q) = %d, %v", s, n, err)
		}
	}
	got := b.String()
	if !strings.HasPrefix(got, "12345678ab\n[the run log reached") || !strings.HasSuffix(got, "for its result]\n") || strings.Count(got, "reached") != 1 {
		t.Fatalf("log = %q", got)
	}
}
