package pipeline

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestPlan_DefinitionOrder(t *testing.T) {
	p := Pipeline{
		Version: "1",
		Name:    "hello-world",
		Steps: []Step{
			{ID: "hello", Run: "echo hello"},
			{ID: "second", Run: "echo second"},
		},
	}
	plan, err := Plan(p)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Name != "hello-world" {
		t.Fatalf("Name = %q", plan.Name)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].ID != "hello" || plan.Steps[1].ID != "second" {
		t.Fatalf("Steps = %+v", plan.Steps)
	}
}

func TestPlan_DependencyOrder(t *testing.T) {
	tests := []struct {
		name  string
		steps []Step
		want  []string
	}{
		{
			name: "earlier step needs a later step",
			steps: []Step{
				{ID: "test", Needs: []string{"build"}},
				{ID: "build"},
			},
			want: []string{"build", "test"},
		},
		{
			name: "step needs two later steps",
			steps: []Step{
				{ID: "package", Needs: []string{"lint", "test"}},
				{ID: "lint"},
				{ID: "test"},
			},
			want: []string{"lint", "test", "package"},
		},
		{
			name: "unrelated last step stays last",
			steps: []Step{
				{ID: "report", Needs: []string{"build"}},
				{ID: "build"},
				{ID: "scan"},
			},
			want: []string{"build", "report", "scan"},
		},
		{
			name: "steps without needs are not ordered after their predecessor",
			steps: []Step{
				{ID: "publish", Needs: []string{"package"}},
				{ID: "lint"},
				{ID: "package", Needs: []string{}},
			},
			want: []string{"lint", "package", "publish"},
		},
		{
			name: "order of ids inside needs does not change run order",
			steps: []Step{
				{ID: "package", Needs: []string{"test", "lint"}},
				{ID: "lint"},
				{ID: "test"},
			},
			want: []string{"lint", "test", "package"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.steps {
				tt.steps[i].Run = "echo " + tt.steps[i].ID
			}
			plan, err := Plan(Pipeline{Version: "1", Name: "n", Steps: tt.steps})
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			var got []string
			for _, s := range plan.Steps {
				got = append(got, s.ID)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("order = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlan_WithoutNeedsKeepsDefinitionOrder(t *testing.T) {
	tests := []struct {
		name  string
		needs []string
	}{
		{name: "needs omitted", needs: nil},
		{name: "needs empty list", needs: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pipeline{Version: "1", Name: "n", Steps: []Step{
				{ID: "c", Run: "echo c", Needs: tt.needs},
				{ID: "a", Run: "echo a", Needs: tt.needs},
				{ID: "b", Run: "echo b", Needs: tt.needs},
			}}
			plan, err := Plan(p)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			var got []string
			for _, s := range plan.Steps {
				got = append(got, s.ID)
			}
			if strings.Join(got, ",") != "c,a,b" {
				t.Fatalf("order = %q, want definition order c,a,b", got)
			}
		})
	}
}

func TestPlan_KeepsOnlyDeclaredNeeds(t *testing.T) {
	steps := []Step{
		{ID: "test", Run: "echo test", Needs: []string{"build"}},
		{ID: "build", Run: "echo build"},
		{ID: "scan", Run: "echo scan"},
	}
	plan, err := Plan(Pipeline{Version: "1", Name: "n", Steps: steps})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	got := map[string][]string{}
	for _, s := range plan.Steps {
		got[s.ID] = s.Needs
	}
	want := map[string][]string{"test": {"build"}, "build": nil, "scan": nil}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Needs = %#v, want %#v", got, want)
	}
}

func TestPlan_DoesNotChangeTheDefinition(t *testing.T) {
	p := Pipeline{Version: "1", Name: "n", Steps: []Step{
		{ID: "test", Run: "echo test", Needs: []string{"build"}},
		{ID: "build", Run: "echo build"},
	}}
	if _, err := Plan(p); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if p.Steps[0].ID != "test" || p.Steps[1].ID != "build" {
		t.Fatalf("definition reordered: %+v", p.Steps)
	}
}

func TestPlan_Concurrency(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "not declared becomes 1", in: 0, want: 1},
		{name: "1 stays 1", in: 1, want: 1},
		{name: "2 is copied", in: 2, want: 2},
		{name: "16 is copied", in: 16, want: 16},
		{name: "maximum means as many as are ready", in: math.MaxInt, want: math.MaxInt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pipeline{Version: "1", Name: "n", Concurrency: tt.in, Steps: []Step{{ID: "a", Run: "echo a"}}}
			plan, err := Plan(p)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			if plan.Concurrency != tt.want {
				t.Fatalf("Concurrency = %d, want %d", plan.Concurrency, tt.want)
			}
		})
	}
}

func TestPlan_DirPrecedenceIsStepThenJobThenWorkdir(t *testing.T) {
	p, err := Parse([]byte(`
version: "1"
pipeline:
  name: dirs
  workdir: base
jobs:
  a:
    dir: jobdir
    steps:
      - id: own
        run: x
        dir: stepdir
      - id: inherit
        run: x
  b:
    steps:
      - id: plain
        run: x
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
	got := map[string]string{}
	for _, s := range plan.Graph {
		got[s.ID] = s.Dir
	}
	want := map[string]string{"a/own": "stepdir", "a/inherit": "jobdir", "b/plain": "base"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirs = %v, want %v", got, want)
	}
}

func TestPlan_WorkdirAppliesToFlatSteps(t *testing.T) {
	p, err := Parse([]byte("version: \"1\"\npipeline:\n  name: n\n  workdir: base\nsteps:\n  - id: a\n    run: x\n  - id: b\n    run: x\n    dir: other\n"))
	if err != nil || Validate(p) != nil {
		t.Fatalf("parse/validate: %v %v", err, Validate(p))
	}
	plan, err := Plan(p)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Graph[0].Dir != "base" || plan.Graph[1].Dir != "other" {
		t.Fatalf("dirs = %q, %q", plan.Graph[0].Dir, plan.Graph[1].Dir)
	}
}

func TestValidate_WorkdirMustBeANonEmptyString(t *testing.T) {
	for _, value := range []string{`""`, "[a]", "5"} {
		p, err := Parse([]byte("version: \"1\"\npipeline:\n  name: n\n  workdir: " + value + "\nsteps:\n  - id: a\n    run: x\n"))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(p); err == nil || !strings.Contains(err.Error(), "pipeline workdir must be a non-empty string") {
			t.Errorf("workdir %s: err = %v", value, err)
		}
	}
}
