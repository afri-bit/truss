package executor

import (
	"strings"
	"testing"
)

func TestCapBuffer_KeepsTheFirstMiB(t *testing.T) {
	var c capBuffer
	chunk := strings.Repeat("x", 300<<10)
	for i := 0; i < 10; i++ {
		if n, err := c.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatalf("Write = %d, %v", n, err)
		}
	}
	got := c.String()
	if len(got) > maxKeptOutput+64 || !strings.HasSuffix(got, "[output cut at 1 MiB]\n") || strings.Count(got, "cut at") != 1 {
		t.Fatalf("len %d, tail %q", len(got), got[len(got)-40:])
	}
}
