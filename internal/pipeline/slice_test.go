package pipeline

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// sliceFlow has two jobs that depend on each other and one that stands alone:
//
//	lint/version -> lint/linux -> test/unit
//	docs/build
func sliceFlow(t *testing.T) Pipeline {
	t.Helper()
	jobIf, err := ParseCondition("env.CI == 'yes'")
	if err != nil {
		t.Fatal(err)
	}
	return Pipeline{
		Version: "1",
		Name:    "flow",
		Env:     map[string]string{"A": "pipeline", "P": "p"},
		Jobs: []Job{
			{ID: "lint", Env: map[string]string{"B": "job"}, Steps: []Step{
				{ID: "version", Run: "v"},
				{ID: "linux", Run: "l", Env: map[string]string{"A": "step"}, Secrets: []string{"TOKEN"}},
			}},
			{ID: "test", Needs: []string{"lint"}, If: jobIf, Steps: []Step{
				{ID: "unit", Run: "u", Retries: 2},
			}},
			{ID: "docs", Steps: []Step{{ID: "build", Run: "d"}}},
		},
	}
}

func ids(p Pipeline) []string {
	out := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		out[i] = s.ID
	}
	return out
}

func TestSlice_Modes(t *testing.T) {
	tests := []struct {
		name string
		sel  Selection
		want []string
	}{
		{"only", Selection{Step: "lint/linux"}, []string{"lint/linux"}},
		{"with needs", Selection{Step: "test/unit", Mode: WithNeeds}, []string{"lint/version", "lint/linux", "test/unit"}},
		{"from", Selection{Step: "lint/linux", Mode: From}, []string{"lint/linux", "test/unit"}},
		{"from the first step", Selection{Step: "lint/version", Mode: From}, []string{"lint/version", "lint/linux", "test/unit"}},
		{"from a last step", Selection{Step: "docs/build", Mode: From}, []string{"docs/build"}},
		{"with needs of a first step", Selection{Step: "lint/version", Mode: WithNeeds}, []string{"lint/version"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Slice(sliceFlow(t), tt.sel)
			if err != nil {
				t.Fatalf("Slice() error = %v", err)
			}
			if !reflect.DeepEqual(ids(got), tt.want) {
				t.Fatalf("steps = %v, want %v", ids(got), tt.want)
			}
			if err := Validate(got); err != nil {
				t.Fatalf("slice does not validate: %v", err)
			}
			if _, err := Plan(got); err != nil {
				t.Fatalf("slice does not plan: %v", err)
			}
		})
	}
}

func TestSlice_NeedsOutsideTheSliceAreDropped(t *testing.T) {
	got, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Mode: From})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Step{}
	for _, s := range got.Steps {
		byID[s.ID] = s
	}
	if n := byID["lint/linux"].Needs; len(n) != 0 {
		t.Errorf("lint/linux needs = %v, want none (lint/version is outside the slice)", n)
	}
	if n := byID["test/unit"].Needs; !reflect.DeepEqual(n, []string{"lint/linux"}) {
		t.Errorf("test/unit needs = %v, want [lint/linux]", n)
	}
}

func TestSlice_StepsKeepTheirMergedSettings(t *testing.T) {
	got, err := Slice(sliceFlow(t), Selection{Step: "lint/linux"})
	if err != nil {
		t.Fatal(err)
	}
	s := got.Steps[0]
	wantEnv := map[string]string{"A": "step", "B": "job", "P": "p"}
	if !reflect.DeepEqual(s.Env, wantEnv) {
		t.Errorf("Env = %v, want %v", s.Env, wantEnv)
	}
	if !reflect.DeepEqual(s.Secrets, []string{"TOKEN"}) {
		t.Errorf("Secrets = %v", s.Secrets)
	}
	unit, err := Slice(sliceFlow(t), Selection{Step: "test/unit"})
	if err != nil {
		t.Fatal(err)
	}
	if unit.Steps[0].Retries != 2 {
		t.Errorf("Retries = %d, want 2", unit.Steps[0].Retries)
	}
}

func TestSlice_TheJobConditionIsNotEvaluated(t *testing.T) {
	got, err := Slice(sliceFlow(t), Selection{Step: "test/unit"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].JobIf != nil {
		t.Errorf("JobIf = %q, want none: naming a step runs it", got.Steps[0].JobIf.String())
	}
}

func TestSlice_TheStepsOwnConditionIsKept(t *testing.T) {
	p := Pipeline{Version: "1", Name: "f", Steps: []Step{{ID: "a", Run: "a"}, {ID: "b", Run: "b", Needs: []string{"a"}}}}
	cond, err := ParseCondition("success()")
	if err != nil {
		t.Fatal(err)
	}
	p.Steps[1].If = cond
	got, err := Slice(p, Selection{Step: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].If == nil || got.Steps[0].If.String() != "success()" {
		t.Errorf("If = %v, want success()", got.Steps[0].If)
	}
}

func TestSlice_KeepsConcurrency(t *testing.T) {
	p := sliceFlow(t)
	p.Concurrency = 3
	got, err := Slice(p, Selection{Step: "test/unit", Mode: WithNeeds})
	if err != nil {
		t.Fatal(err)
	}
	if got.Concurrency != 3 || got.Name != "flow" {
		t.Errorf("Concurrency = %d, Name = %q", got.Concurrency, got.Name)
	}
}

func TestSlice_EnvOverrideWinsOverEveryLayer(t *testing.T) {
	got, err := Slice(sliceFlow(t), Selection{
		Step: "lint/linux", Mode: From,
		Env: map[string]string{"A": "override", "NEW": "n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range got.Steps {
		if s.Env["A"] != "override" || s.Env["NEW"] != "n" {
			t.Errorf("%s Env = %v", s.ID, s.Env)
		}
	}
	if got.Steps[0].Env["B"] != "job" {
		t.Errorf("an override must not drop other variables: %v", got.Steps[0].Env)
	}
}

func TestSlice_EnvOverrideDoesNotChangeTheInput(t *testing.T) {
	p := sliceFlow(t)
	if _, err := Slice(p, Selection{Step: "lint/linux", Env: map[string]string{"A": "x"}}); err != nil {
		t.Fatal(err)
	}
	if p.Env["A"] != "pipeline" || p.Jobs[0].Steps[1].Env["A"] != "step" {
		t.Errorf("the pipeline was changed: %v %v", p.Env, p.Jobs[0].Steps[1].Env)
	}
}

func TestSlice_RefusesToOverrideASecret(t *testing.T) {
	_, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Env: map[string]string{"TOKEN": "x"}})
	if err == nil || !strings.Contains(err.Error(), "TOKEN") || !strings.Contains(err.Error(), "truss secret set") {
		t.Fatalf("error = %v, want it to name TOKEN and truss secret set", err)
	}
	// A secret of a step that is not in the slice is not a problem.
	if _, err := Slice(sliceFlow(t), Selection{Step: "test/unit", Env: map[string]string{"TOKEN": "x"}}); err != nil {
		t.Fatalf("TOKEN is not a secret of test/unit: %v", err)
	}
}

func TestSlice_UnknownStep(t *testing.T) {
	_, err := Slice(sliceFlow(t), Selection{Step: "lint/missing"})
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{`"lint/missing"`, "lint/version", "docs/build"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestSlice_FlatPipeline(t *testing.T) {
	p := Pipeline{Version: "1", Name: "f", Env: map[string]string{"X": "1"}, Workdir: "sub", Steps: []Step{
		{ID: "build", Run: "b"},
		{ID: "test", Run: "t", Needs: []string{"build"}},
	}}
	got, err := Slice(p, Selection{Step: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(got), []string{"test"}) || got.Steps[0].Dir != "sub" || got.Steps[0].Env["X"] != "1" {
		t.Errorf("slice = %+v", got.Steps)
	}
}

func TestPlan_RecordsWhereEachVariableComesFrom(t *testing.T) {
	plan, err := Plan(sliceFlow(t))
	if err != nil {
		t.Fatal(err)
	}
	var linux Step
	for _, s := range plan.Graph {
		if s.ID == "lint/linux" {
			linux = s
		}
	}
	want := map[string]string{"A": "step", "B": "job lint", "P": "pipeline"}
	if !reflect.DeepEqual(linux.EnvSource, want) {
		t.Errorf("EnvSource = %v, want %v", linux.EnvSource, want)
	}
}

func TestPlan_NoEnvNoSource(t *testing.T) {
	plan, err := Plan(Pipeline{Version: "1", Name: "f", Steps: []Step{{ID: "a", Run: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Graph[0].EnvSource != nil {
		t.Errorf("EnvSource = %v, want nil", plan.Graph[0].EnvSource)
	}
}

func TestPlan_NeededBy(t *testing.T) {
	plan, err := Plan(sliceFlow(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.NeededBy("lint/version"); !reflect.DeepEqual(got, []string{"lint/linux"}) {
		t.Errorf("NeededBy(lint/version) = %v", got)
	}
	if got := plan.NeededBy("lint/linux"); !reflect.DeepEqual(got, []string{"test/unit"}) {
		t.Errorf("NeededBy(lint/linux) = %v", got)
	}
	if got := plan.NeededBy("docs/build"); len(got) != 0 {
		t.Errorf("NeededBy(docs/build) = %v", got)
	}
}

func TestSlice_NoStepMeansTheWholePipeline(t *testing.T) {
	got, err := Slice(sliceFlow(t), Selection{Env: map[string]string{"A": "o"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 4 {
		t.Fatalf("steps = %v", ids(got))
	}
	for _, s := range got.Steps {
		if s.Env["A"] != "o" {
			t.Errorf("%s Env = %v", s.ID, s.Env)
		}
		if s.ID == "test/unit" && s.JobIf == nil {
			t.Error("a whole run must keep the condition of a job")
		}
	}
	if _, err := Slice(sliceFlow(t), Selection{Env: map[string]string{"TOKEN": "o"}}); err == nil {
		t.Error("a secret may not be overridden in a whole run either")
	}
}

func TestSlice_OtherJobsKeepTheirCondition(t *testing.T) {
	// Only the job of the named step loses its condition. A step pulled in by
	// --from keeps the guard of its job: the user did not name it.
	from, err := Slice(sliceFlow(t), Selection{Step: "lint/version", Mode: From})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range from.Steps {
		if s.ID == "test/unit" && (s.JobIf == nil || s.JobIf.String() != "env.CI == 'yes'") {
			t.Errorf("--from dropped the condition of job test: %v", s.JobIf)
		}
	}
	// With the needs of a named step, the named job's own condition goes and
	// the conditions of the jobs it needs stay.
	guarded := sliceFlow(t)
	cond, err := ParseCondition("env.LINT == 'yes'")
	if err != nil {
		t.Fatal(err)
	}
	guarded.Jobs[0].If = cond
	got, err := Slice(guarded, Selection{Step: "test/unit", Mode: WithNeeds})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range got.Steps {
		switch s.Job {
		case "test":
			if s.JobIf != nil {
				t.Errorf("%s: the named job's condition must go", s.ID)
			}
		case "lint":
			if s.JobIf == nil {
				t.Errorf("%s: the condition of a job that was not named must stay", s.ID)
			}
		}
	}
}

func TestSlice_NamesFoldOnWindows(t *testing.T) {
	foldEnvNames = true
	t.Cleanup(func() { foldEnvNames = runtime.GOOS == "windows" })
	got, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Env: map[string]string{"a": "lower", "p": "q"}})
	if err != nil {
		t.Fatal(err)
	}
	env := got.Steps[0].Env
	if env["a"] != "lower" || env["p"] != "q" {
		t.Errorf("Env = %v", env)
	}
	if _, ok := env["A"]; ok {
		t.Errorf("the host would see A and a as one variable, and the file's A must not win: %v", env)
	}
	if _, ok := env["P"]; ok {
		t.Errorf("P: %v", env)
	}
	if _, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Env: map[string]string{"token": "x"}}); err == nil {
		t.Error("a secret may not be overridden under another case on Windows")
	}
	if EnvKey("Path") != "PATH" {
		t.Errorf("EnvKey = %q", EnvKey("Path"))
	}
}

func TestSlice_NamesDoNotFoldElsewhere(t *testing.T) {
	foldEnvNames = false
	t.Cleanup(func() { foldEnvNames = runtime.GOOS == "windows" })
	got, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Env: map[string]string{"a": "lower"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].Env["A"] != "step" || got.Steps[0].Env["a"] != "lower" {
		t.Errorf("Env = %v: a and A are two variables here", got.Steps[0].Env)
	}
	if _, err := Slice(sliceFlow(t), Selection{Step: "lint/linux", Env: map[string]string{"token": "x"}}); err != nil {
		t.Errorf("token is not TOKEN here: %v", err)
	}
}
