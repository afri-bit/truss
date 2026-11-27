package pipeline

import (
	"strings"
	"testing"
)

// Spec 024: the flow block is called flow; pipeline stays accepted.
func TestParse_FlowKeyIsTheFlowBlock(t *testing.T) {
	for _, key := range []string{"flow", "pipeline"} {
		t.Run(key, func(t *testing.T) {
			src := "version: \"1\"\n" + key + ":\n  name: nightly\n  concurrency: 2\n  workdir: app\n  overlap: skip\nsteps:\n  - id: a\n    run: echo a\n"
			p, err := Parse([]byte(src))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if err := Validate(p); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if p.Name != "nightly" || p.Workdir != "app" || p.Overlap != OverlapSkip {
				t.Fatalf("got name %q workdir %q overlap %q", p.Name, p.Workdir, p.Overlap)
			}
		})
	}
}

func TestValidate_RejectsBothFlowAndPipeline(t *testing.T) {
	src := "version: \"1\"\nflow: {name: a}\npipeline: {name: b}\nsteps:\n  - id: a\n    run: echo a\n"
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err = Validate(p)
	if err == nil || !strings.Contains(err.Error(), "both flow and pipeline") {
		t.Fatalf("Validate() error = %v, want both flow and pipeline", err)
	}
}

func TestValidate_UnknownFieldInTheFlowBlockSaysFlow(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\nflow: {name: a, concurency: 2}\nsteps:\n  - id: a\n    run: echo a\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := Validate(p); err == nil || err.Error() != `unknown field "concurency" in flow` {
		t.Fatalf("Validate() error = %v", err)
	}
}

// v0.2.0 accepted a repeated pipeline block (the last one wins); that stays.
func TestValidate_ARepeatedBlockOfOneNameStaysValid(t *testing.T) {
	for _, key := range []string{"flow", "pipeline"} {
		p, err := Parse([]byte("version: \"1\"\n" + key + ": {name: a}\n" + key + ": {name: b}\nsteps:\n  - id: a\n    run: echo a\n"))
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if err := Validate(p); err != nil || p.Name != "b" {
			t.Fatalf("%s twice: Validate() = %v, name %q", key, err, p.Name)
		}
	}
}

func TestValidate_RejectsBothInEitherOrder(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\npipeline: {name: a}\nflow: {name: b}\nsteps:\n  - id: a\n    run: echo a\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if err := Validate(p); err == nil || !strings.Contains(err.Error(), "both pipeline and flow") {
		t.Fatalf("Validate() error = %v", err)
	}
}
