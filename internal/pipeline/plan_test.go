package pipeline

import "testing"

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
