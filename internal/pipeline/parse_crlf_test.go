package pipeline

import (
	"strings"
	"testing"
)

// A pipeline file saved with Windows line endings parses and validates.
func TestParse_AcceptsCRLF(t *testing.T) {
	text := "version: \"1\"\npipeline:\n  name: crlf\nsteps:\n  - id: a\n    run: echo a\n  - id: b\n    run: echo b\n    needs: [a]\n"
	p, err := Parse([]byte(strings.ReplaceAll(text, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	if p.Name != "crlf" || len(p.Steps) != 2 || p.Steps[1].Run != "echo b" {
		t.Fatalf("pipeline = %+v", p)
	}
}
