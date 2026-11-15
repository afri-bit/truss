package pipeline

import (
	"strings"
	"testing"
	"time"
)

const stepsHead = "version: \"1\"\npipeline:\n  name: n\nsteps:\n"

func TestParse_RetryContinueAndIfFields(t *testing.T) {
	p, err := Parse([]byte(stepsHead + `  - id: a
    run: echo a
    retries: 3
    retry_delay: 1m30s
    continue-on-error: true
    if: failure() && env.STAGE != 'dev'
  - id: b
    run: echo b
    retries: 0
    continue-on-error: false
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	a, b := p.Steps[0], p.Steps[1]
	if a.Retries != 3 || a.RetryDelay != 90*time.Second || !a.ContinueOnError || a.If == nil || a.If.String() != "failure() && env.STAGE != 'dev'" {
		t.Fatalf("a = %+v", a)
	}
	if b.Retries != 0 || b.ContinueOnError || b.If != nil {
		t.Fatalf("b = %+v", b)
	}
}

func TestParse_JobIfAndPlanCarriesIt(t *testing.T) {
	p, err := Parse([]byte(`version: "1"
pipeline:
  name: n
jobs:
  build:
    steps:
      - { id: one, run: echo 1 }
  cleanup:
    needs: [build]
    if: always()
    steps:
      - { id: one, run: echo 1 }
      - { id: two, run: echo 2, if: failure() }
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(p)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range plan.Graph {
		got = append(got, s.ID+":"+s.Job+":"+map[bool]string{true: "first", false: "later"}[s.JobFirst]+":"+s.JobIf.String())
	}
	want := "build/one:build:first:,cleanup/one:cleanup:first:always(),cleanup/two:cleanup:later:always()"
	if strings.Join(got, ",") != want {
		t.Fatalf("plan steps = %v", got)
	}
	if plan.Graph[2].If.String() != "failure()" {
		t.Fatalf("step if = %q", plan.Graph[2].If)
	}
}

func TestValidate_RejectsBadRetryContinueAndIf(t *testing.T) {
	tests := []struct{ name, step, want string }{
		{"negative retries", "retries: -1", `step "a" retries must be a whole number from 0 to 100, got "-1"`},
		{"too many retries", "retries: 101", `retries must be a whole number from 0 to 100, got "101"`},
		{"text retries", "retries: three", `retries must be a whole number`},
		{"float retries", "retries: 1.5", `retries must be a whole number`},
		{"list retries", "retries: [1]", `retries must be a whole number`},
		{"bad delay", "retries: 1\n    retry_delay: soon", `step "a" retry_delay must be a duration such as 90s, 5m or 1h30m, got "soon"`},
		{"zero delay", "retries: 1\n    retry_delay: 0s", `retry_delay must be a duration`},
		{"delay without retries", "retry_delay: 5s", `step "a" retry_delay needs retries of at least 1`},
		{"text continue", "continue-on-error: sometimes", `step "a" continue-on-error must be true or false, got "sometimes"`},
		{"quoted continue", `continue-on-error: "true"`, `continue-on-error must be true or false`},
		{"bool if", "if: true", `step "a" if must be an expression in a string, got "true"`},
		{"empty if", `if: ""`, `step "a" if: the expression is empty`},
		{"bad if", "if: sometimes()", `step "a" if: unknown function "sometimes"`},
		{"underscore spelling", "continue_on_error: true", `unknown field "continue_on_error" in step "a"`},
	}
	for _, tt := range tests {
		p, err := Parse([]byte(stepsHead + "  - id: a\n    run: echo a\n    " + tt.step + "\n"))
		if err != nil {
			t.Fatalf("%s: Parse: %v", tt.name, err)
		}
		err = Validate(p)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want it to contain %q", tt.name, err, tt.want)
		}
	}
}

func TestValidate_RejectsBadJobIf(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\npipeline:\n  name: n\njobs:\n  j:\n    if: nope\n    steps:\n      - { id: a, run: echo a }\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(p)
	if err == nil || !strings.Contains(err.Error(), `job "j" if:`) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidate_TimeoutMessageIsUnchanged(t *testing.T) {
	p, err := Parse([]byte(stepsHead + "  - id: a\n    run: echo a\n    timeout: soon\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := `step "a" timeout must be a duration such as 90s, 5m or 1h30m, got "soon"`
	if err := Validate(p); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}
