package pipeline

import (
	"strings"
	"testing"
	"time"
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

// Spec 028: kill_timeout on the flow is the default of its steps; at most a
// minute.
func TestParse_KillTimeout(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\nflow: {name: f, kill_timeout: 20s}\nsteps:\n  - id: a\n    run: echo a\n  - id: b\n    run: echo b\n    kill_timeout: 2s\n"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(p)
	if err != nil || Validate(p) != nil {
		t.Fatalf("plan: %v %v", err, Validate(p))
	}
	if plan.Graph[0].KillTimeout != 20*time.Second || plan.Graph[1].KillTimeout != 2*time.Second {
		t.Fatalf("kill timeouts = %s, %s", plan.Graph[0].KillTimeout, plan.Graph[1].KillTimeout)
	}
	for _, bad := range []string{"0s", "2m", "-1s", "soon"} {
		p, _ := Parse([]byte("version: \"1\"\nflow: {name: f}\nsteps:\n  - id: a\n    run: echo a\n    kill_timeout: " + bad + "\n"))
		if Validate(p) == nil {
			t.Errorf("kill_timeout %s was accepted", bad)
		}
	}
}
