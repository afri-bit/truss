package pipeline

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

const fullDefinition = `
version: "1"
pipeline:
  name: nightly
  concurrency: 3
  overlap: skip
on:
  schedule:
    - cron: "0 9 * * 1-5"
      timezone: Europe/Berlin
    - cron: "@daily"
      missed: run
env:
  STAGE: dev
  RETRIES: 3
jobs:
  build:
    env:
      STAGE: build
    steps:
      - id: compile
        run: make
        timeout: 1m30s
      - id: package
        run: make dist
        env:
          STAGE: package
  lint:
    dir: tools
    steps:
      - id: vet
        run: go vet ./...
        dir: elsewhere
      - id: fmt
        run: gofmt -l .
  release:
    needs: [build, lint]
    steps:
      - id: publish
        run: make publish
`

func mustParse(t *testing.T, text string) Pipeline {
	t.Helper()
	p, err := Parse([]byte(text))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return p
}

func TestParse_FullDefinition(t *testing.T) {
	p := mustParse(t, fullDefinition)
	if err := Validate(p); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.Overlap != OverlapSkip || p.Concurrency != 3 {
		t.Errorf("overlap = %q, concurrency = %d", p.Overlap, p.Concurrency)
	}
	want := []Schedule{
		{Cron: "0 9 * * 1-5", Timezone: "Europe/Berlin", Missed: MissedSkip},
		{Cron: "@daily", Missed: MissedRun},
	}
	if !reflect.DeepEqual(p.Schedules, want) {
		t.Errorf("schedules = %+v", p.Schedules)
	}
	if p.Env["RETRIES"] != "3" {
		t.Errorf("env = %v", p.Env)
	}
	if len(p.Jobs) != 3 || p.Jobs[2].ID != "release" {
		t.Fatalf("jobs = %+v", p.Jobs)
	}
}

func TestPlan_FlattensJobs(t *testing.T) {
	plan, err := Plan(mustParse(t, fullDefinition))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, needs, stage, dir string
		timeout               time.Duration
	}
	var got []row
	for _, s := range plan.Graph {
		got = append(got, row{s.ID, strings.Join(s.Needs, ","), s.Env["STAGE"], s.Dir, s.Timeout})
	}
	want := []row{
		{"build/compile", "", "build", "", 90 * time.Second},
		{"build/package", "build/compile", "package", "", 0},
		{"lint/vet", "", "dev", "elsewhere", 0},
		{"lint/fmt", "lint/vet", "dev", "tools", 0},
		{"release/publish", "build/package,lint/fmt", "dev", "", 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("graph =\n%+v\nwant\n%+v", got, want)
	}
	if len(plan.Steps) != len(want) || plan.Concurrency != 3 {
		t.Fatalf("steps = %d, concurrency = %d", len(plan.Steps), plan.Concurrency)
	}
}

func TestPlan_TopLevelStepsInheritPipelineEnv(t *testing.T) {
	plan, err := Plan(mustParse(t, `
version: "1"
pipeline: {name: x}
env: {A: "1", B: "2"}
steps:
  - {id: one, run: "true", env: {B: "3"}}
`))
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Graph[0].Env; got["A"] != "1" || got["B"] != "3" {
		t.Fatalf("env = %v", got)
	}
}

func TestValidate_NewFieldFaults(t *testing.T) {
	head := "version: \"1\"\npipeline: {name: x}\n"
	steps := "steps:\n  - {id: a, run: \"true\"}\n"
	tests := []struct{ name, body, want string }{
		{"overlap", "version: \"1\"\npipeline: {name: x, overlap: sometimes}\n" + steps, `pipeline overlap must be one of queue, skip, parallel, got "sometimes"`},
		{"on not mapping", head + "on: push\n" + steps, "on must be a mapping"},
		{"unknown trigger", head + "on: {push: true}\n" + steps, `unknown field "push" in on`},
		{"empty schedule", head + "on: {schedule: []}\n" + steps, "on.schedule must be a list with at least one schedule"},
		{"bad cron", head + "on: {schedule: [{cron: \"61 * * * *\"}]}\n" + steps, "schedule 1: cron"},
		{"no cron", head + "on: {schedule: [{missed: run}]}\n" + steps, "schedule 1 is missing cron"},
		{"bad timezone", head + "on: {schedule: [{cron: \"@daily\", timezone: Mars/Olympus}]}\n" + steps, `schedule 1: unknown timezone "Mars/Olympus"`},
		{"bad missed", head + "on: {schedule: [{cron: \"@daily\", missed: later}]}\n" + steps, "schedule 1 missed must be one of skip, run"},
		{"env name", head + "env: {\"1A\": x}\n" + steps, `env: "1A" is not a valid variable name`},
		{"env value", head + "env: {A: [1]}\n" + steps, "env: value of A must be a single value"},
		{"step timeout", head + "steps:\n  - {id: a, run: \"true\", timeout: soon}\n", `step "a" timeout must be a duration such as 90s, 5m or 1h30m, got "soon"`},
		{"step dir", head + "steps:\n  - {id: a, run: \"true\", dir: \"\"}\n", `step "a" dir must be a non-empty string`},
		{"both", head + steps + "jobs: {j: {steps: [{id: a, run: \"true\"}]}}\n", "a pipeline has either steps or jobs, not both"},
		{"no jobs", head + "jobs: {}\n", "pipeline must contain one or more jobs"},
		{"job id", head + "jobs: {\"a/b\": {steps: [{id: a, run: \"true\"}]}}\n", `job id "a/b" must use only`},
		{"empty job", head + "jobs: {j: {steps: []}}\n", `job "j" must contain one or more steps`},
		{"job unknown field", head + "jobs: {j: {when: x, steps: [{id: a, run: \"true\"}]}}\n", `unknown field "when" in job "j"`},
		{"job step needs", head + "jobs: {j: {steps: [{id: a, run: \"true\", needs: [b]}]}}\n", `step "j/a" cannot declare needs`},
		{"job step run", head + "jobs: {j: {steps: [{id: a}]}}\n", `missing or empty run for step "j/a"`},
		{"job dup step", head + "jobs: {j: {steps: [{id: a, run: x}, {id: a, run: x}]}}\n", `duplicate step id "j/a"`},
		{"job unknown need", head + "jobs: {j: {needs: [k], steps: [{id: a, run: x}]}}\n", `job "j" needs unknown job "k"`},
		{"job cycle", head + "jobs:\n  j: {needs: [k], steps: [{id: a, run: x}]}\n  k: {needs: [j], steps: [{id: a, run: x}]}\n", "dependency cycle: j, k"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(mustParse(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
