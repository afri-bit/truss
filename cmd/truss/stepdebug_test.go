package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Spec 023: inspect one step and run a slice of a flow.

// dbgFlow has a job with two steps, a job after it, and variables of every kind the step
// view tells apart.
const dbgFlow = `version: "1"
pipeline: {name: dbg}
env: {REGION: eu, API_TOKEN: abc-do-not-print, MODE: pipeline}
jobs:
  build:
    env: {MODE: job}
    steps:
      - {id: compile, run: "echo compile"}
      - id: package
        env: {MODE: step}
        secrets: [NPM_TOKEN]
        run: |
          echo pack $REGION $UNSET_VAR ${HOME_DIR:-x} %WIN_VAR% $MY_API_TOKEN $OTHER
          for ITEM in a b; do echo $ITEM; done
          LOCAL_ONE=1; echo $LOCAL_ONE; echo $lowercase
  ship:
    needs: [build]
    steps:
      - {id: push, run: "echo push", timeout: 5m, retries: 2}
`

func debugSession(t *testing.T) *session {
	t.Helper()
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/dbg.yaml", dbgFlow)
	return s
}

// row finds the table row that starts with name and returns it.
func row(t *testing.T, out, name string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) > 0 && f[0] == name {
			return l
		}
	}
	t.Fatalf("no row %s in:\n%s", name, out)
	return ""
}

func wantRow(t *testing.T, out, name string, cells ...string) {
	t.Helper()
	got := row(t, out, name)
	re := regexp.MustCompile(`^` + strings.Join(cells, `\s+`) + `\s*$`)
	if !re.MatchString(got) {
		t.Errorf("row %s = %q, want cells %q", name, got, cells)
	}
}

func TestStepInspect_ShowsTheTopography(t *testing.T) {
	s := debugSession(t)
	out := s.ok("flows", "show", "dbg", "build/package")
	for _, want := range []string{"build/package", "build/compile", "ship/push", "echo pack"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !regexp.MustCompile(`(?m)^Needs\s+build/compile$`).MatchString(out) || !regexp.MustCompile(`(?m)^Needed by\s+ship/push$`).MatchString(out) {
		t.Errorf("needs and needed by are wrong:\n%s", out)
	}
	push := s.ok("flows", "show", "dbg", "ship/push")
	for _, want := range []string{"5m0s", "Retries"} {
		if !strings.Contains(push, want) {
			t.Errorf("ship/push lacks %q:\n%s", want, push)
		}
	}
}

func TestStepInspect_EnvironmentTable(t *testing.T) {
	s := debugSession(t)
	out := s.ok("flows", "show", "dbg", "build/package")
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "SOURCE") || !strings.Contains(out, "STATE") || !strings.Contains(out, "VALUE") {
		t.Errorf("no table header:\n%s", out)
	}
	wantRow(t, out, "REGION", "REGION", "pipeline", "set", "eu")
	wantRow(t, out, "MODE", "MODE", "step", "set", "step") // the step wins over the job and the pipeline
	wantRow(t, out, "NPM_TOKEN", "NPM_TOKEN", `secret store`, "missing", "-")
	wantRow(t, out, "TRUSS_DATA_DIR", "TRUSS_DATA_DIR", `truss run`, "set", `\.truss\S*dbg\S*data`)
	wantRow(t, out, "TRUSS_RUN_ID", "TRUSS_RUN_ID", "daemon", `daemon only`, "-")
	wantRow(t, out, "UNSET_VAR", "UNSET_VAR", `host environment`, `not set`, "-")
	wantRow(t, out, "HOME_DIR", "HOME_DIR", `host environment`, `not set`, "-")
	wantRow(t, out, "WIN_VAR", "WIN_VAR", `host environment`, `not set`, "-")
	for _, not := range []string{"ITEM", "LOCAL_ONE", "lowercase"} {
		for _, l := range strings.Split(out, "\n") {
			if f := strings.Fields(l); len(f) > 0 && f[0] == not {
				t.Errorf("%s is not an environment variable the step needs: %q", not, l)
			}
		}
	}
	if strings.Contains(out, "abc-do-not-print") {
		t.Errorf("a variable named like a token was printed:\n%s", out)
	}
	wantRow(t, out, "API_TOKEN", "API_TOKEN", "pipeline", "set", `\*\*\*`)
}

func TestStepInspect_JobSourceIsNamed(t *testing.T) {
	s := debugSession(t)
	out := s.ok("flows", "show", "dbg", "build/compile")
	wantRow(t, out, "MODE", "MODE", `job build`, "set", "job")
}

func TestStepInspect_SecretsAreSetOrMissingNeverTheValue(t *testing.T) {
	s := debugSession(t)
	s.setSecret("NPM_TOKEN", "npm-secret-value")
	for _, args := range [][]string{
		{"flows", "show", "dbg", "build/package"},
		{"flows", "show", "dbg", "build/package", "--values"},
		{"flows", "show", "dbg", "build/package", "--json"},
	} {
		out := s.ok(args...)
		if strings.Contains(out, "npm-secret-value") {
			t.Fatalf("%v printed a secret:\n%s", args, out)
		}
	}
	out := s.ok("flows", "show", "dbg", "build/package")
	wantRow(t, out, "NPM_TOKEN", "NPM_TOKEN", `secret store`, "set", `\*\*\*`)
}

func TestStepInspect_HostValuesOnlyWithValues(t *testing.T) {
	s := debugSession(t)
	s.setSecret("NPM_TOKEN", "stored-value")
	s.env = append(s.env, "UNSET_VAR=visible-value", "MY_API_TOKEN=token-value", "OTHER=stored-value")
	plain := s.ok("flows", "show", "dbg", "build/package")
	wantRow(t, plain, "UNSET_VAR", "UNSET_VAR", `host environment`, "set", "-")
	if strings.Contains(plain, "visible-value") {
		t.Errorf("a host value was printed without --values:\n%s", plain)
	}
	shown := s.ok("flows", "show", "dbg", "build/package", "--values")
	wantRow(t, shown, "UNSET_VAR", "UNSET_VAR", `host environment`, "set", "visible-value")
	wantRow(t, shown, "MY_API_TOKEN", "MY_API_TOKEN", `host environment`, "set", `\*\*\*`)
	wantRow(t, shown, "OTHER", "OTHER", `host environment`, "set", `\*\*\*`)
	for _, v := range []string{"token-value", "stored-value"} {
		if strings.Contains(shown, v) {
			t.Errorf("--values printed %q:\n%s", v, shown)
		}
	}
}

func TestStepInspect_JSON(t *testing.T) {
	s := debugSession(t)
	s.env = append(s.env, "UNSET_VAR=visible-value")
	raw := s.ok("flows", "show", "dbg", "build/package", "--json")
	d := decode[stepDoc](t, raw)
	if d.Flow != "dbg" || d.Step.ID != "build/package" || d.Step.Job != "build" || !strings.Contains(d.Step.Run, "echo pack") {
		t.Fatalf("doc = %+v", d.Step)
	}
	if strings.Join(d.Step.Needs, ",") != "build/compile" || strings.Join(d.Step.NeededBy, ",") != "ship/push" {
		t.Errorf("needs = %v, needed_by = %v", d.Step.Needs, d.Step.NeededBy)
	}
	if d.Step.RunDir == "" {
		t.Error("run_dir is empty")
	}
	got := map[string]envRow{}
	for _, r := range d.Environment {
		got[r.Name] = r
	}
	if r := got["REGION"]; r.Kind != "declared" || r.Source != "pipeline" || r.State != "set" || r.Value == nil || *r.Value != "eu" {
		t.Errorf("REGION = %+v", r)
	}
	if r := got["NPM_TOKEN"]; r.Kind != "secret" || r.State != "missing" || r.Value != nil {
		t.Errorf("NPM_TOKEN = %+v", r)
	}
	if r := got["API_TOKEN"]; r.Value == nil || *r.Value != "***" {
		t.Errorf("API_TOKEN = %+v", r)
	}
	if r := got["TRUSS_RUN_ID"]; r.Kind != "truss" || r.State != "daemon-only" || r.Value != nil {
		t.Errorf("TRUSS_RUN_ID = %+v", r)
	}
	if r := got["UNSET_VAR"]; r.Kind != "referenced" || r.State != "set" || r.Value != nil {
		t.Errorf("UNSET_VAR = %+v, the value needs --values", r)
	}
	if strings.Contains(raw, "visible-value") || strings.Contains(raw, "abc-do-not-print") {
		t.Errorf("json prints a value it must not:\n%s", raw)
	}
	compile := decode[stepDoc](t, s.ok("flows", "show", "dbg", "build/compile", "--json"))
	if compile.Step.Needs == nil || len(compile.Step.Needs) != 0 {
		t.Errorf("needs = %#v, want []", compile.Step.Needs)
	}
}

func TestStepInspect_UnknownStepListsTheIDs(t *testing.T) {
	s := debugSession(t)
	code, out, errOut := s.truss("flows", "show", "dbg", "build/nope")
	if code != 1 || out != "" {
		t.Fatalf("exit = %d, stdout = %q", code, out)
	}
	for _, want := range []string{`"build/nope"`, "build/compile", "build/package", "ship/push"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q: %s", want, errOut)
		}
	}
}

func TestStepInspect_ChangesNothing(t *testing.T) {
	s := debugSession(t)
	before := tree(t, s.dir)
	s.ok("flows", "show", "dbg", "build/package")
	s.ok("flows", "show", "dbg", "build/package", "--json")
	if after := tree(t, s.dir); after != before {
		t.Errorf("the project changed:\nbefore: %s\nafter:  %s", before, after)
	}
}

func tree(t *testing.T, dir string) string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		names = append(names, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(names, "\n")
}

func TestStepInspect_ControlCharactersBecomeQuestionMarks(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/evil.yaml", "version: \"1\"\nsteps:\n  - {id: a, run: \"echo \\e]0;pwned\\a done\"}\n")
	out := s.ok("flows", "show", "evil", "a")
	if strings.ContainsRune(out, 0x1b) || strings.ContainsRune(out, 0x07) {
		t.Errorf("a control character reached the screen: %q", out)
	}
}

func TestStepInspect_TheFlowViewIsUnchanged(t *testing.T) {
	s := debugSession(t)
	out := s.ok("flows", "show", "dbg")
	if !strings.Contains(out, "(names only)") || strings.Contains(out, "abc-do-not-print") {
		t.Errorf("flows show changed:\n%s", out)
	}
	if code, _, _ := s.truss("flows", "show", "dbg", "a", "b"); code != 1 {
		t.Errorf("two extra arguments: exit = %d, want usage", code)
	}
}

// twoSteps is a flow whose steps print a marker and their name.
func twoSteps() string {
	return `version: "1"
pipeline: {name: two}
steps:
  - {id: a, run: "echo marker-a"}
  - {id: b, run: "echo marker-b", needs: [a]}
  - {id: c, run: "echo marker-c", needs: [b]}
`
}

func twoSession(t *testing.T) *session {
	t.Helper()
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/two.yaml", twoSteps())
	return s
}

func TestDebugRun_OnlyTheNamedStep(t *testing.T) {
	s := twoSession(t)
	code, out, errOut := s.truss("run", "two", "--step", "b")
	if code != 0 {
		t.Fatalf("exit = %d: %s", code, errOut)
	}
	if !strings.Contains(out, "marker-b") || strings.Contains(out, "marker-a") || strings.Contains(out, "marker-c") {
		t.Errorf("wrong steps ran:\n%s", out)
	}
	if !strings.Contains(errOut, "debug run: 1 of 3 steps (b); the run log is not saved") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestDebugRun_WithNeedsAndFrom(t *testing.T) {
	s := twoSession(t)
	_, out, _ := s.truss("run", "two", "--step=c", "--with-needs")
	for _, m := range []string{"marker-a", "marker-b", "marker-c"} {
		if !strings.Contains(out, m) {
			t.Errorf("--with-needs lacks %s:\n%s", m, out)
		}
	}
	_, out, errOut := s.truss("run", "two", "--from", "b")
	if strings.Contains(out, "marker-a") || !strings.Contains(out, "marker-b") || !strings.Contains(out, "marker-c") {
		t.Errorf("--from b:\n%s", out)
	}
	if !strings.Contains(errOut, "2 of 3 steps (b, c)") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestDebugRun_LeavesTheLastRunLogAlone(t *testing.T) {
	s := twoSession(t)
	s.ok("run", "two")
	before := stateLog(t, s.dir, "two")
	s.ok("run", "two", "--step", "b")
	s.ok("run", "two", "--from", "c")
	if after := stateLog(t, s.dir, "two"); after != before {
		t.Errorf("a debug run replaced run.log:\n%s", after)
	}
}

func TestDebugRun_WritesNoLogWhenThereWasNone(t *testing.T) {
	s := twoSession(t)
	s.ok("run", "two", "--step", "a")
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "state", "two", "run.log")); !os.IsNotExist(err) {
		t.Errorf("run.log exists after a debug run: %v", err)
	}
}

func TestDebugRun_UsesTheDataFolder(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/data.yaml", "version: \"1\"\nsteps:\n  - id: show\n    run: '"+helperCmd("print-env", "TRUSS_DATA_DIR")+"'\n")
	out := s.ok("run", "data", "--step", "show")
	if !strings.Contains(out, filepath.Join(".truss", "state", "data", "data")) {
		t.Errorf("TRUSS_DATA_DIR is not the flow's data folder:\n%s", out)
	}
}

func TestDebugRun_EnvOverride(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/envf.yaml", "version: \"1\"\nenv: {FROM_FILE: file}\nsteps:\n"+
		"  - id: show\n    run: '"+helperCmd("print-env", "FROM_FILE")+"'\n"+
		"  - id: more\n    needs: [show]\n    run: '"+helperCmd("print-env", "ADDED")+"'\n")
	fileValue := regexp.MustCompile(`(?m)\bfile\s*$`)
	if base := s.ok("run", "envf", "--step", "show"); !fileValue.MatchString(base) {
		t.Fatalf("the file's value is not printed:\n%s", base)
	}
	out := s.ok("run", "envf", "--step", "show", "--env", "FROM_FILE=override")
	if !strings.Contains(out, "override") || fileValue.MatchString(out) {
		t.Errorf("the override did not win:\n%s", out)
	}
	out = s.ok("run", "envf", "--from", "more", "--env=ADDED=new-value")
	if !strings.Contains(out, "new-value") {
		t.Errorf("an added variable is missing:\n%s", out)
	}
	// --env alone is a whole, normal run: it saves its log.
	s.ok("run", "envf", "--env", "FROM_FILE=whole")
	if log := stateLog(t, s.dir, "envf"); !strings.Contains(log, "whole") {
		t.Errorf("the whole run did not save its log:\n%s", log)
	}
}

func TestDebugRun_EnvOverrideValuesStayOutOfTheLog(t *testing.T) {
	s := twoSession(t)
	_, out, errOut := s.truss("run", "two", "--step", "a", "--env", "SOMETHING=hush-hush-value")
	if strings.Contains(out+errOut, "hush-hush-value") {
		t.Errorf("the override value was printed:\n%s\n%s", out, errOut)
	}
}

func TestDebugRun_RefusesToOverrideASecret(t *testing.T) {
	s := debugSession(t)
	code, out, errOut := s.truss("run", "dbg", "--step", "build/package", "--env", "NPM_TOKEN=x")
	if code != 1 || out != "" || !strings.Contains(errOut, "truss secret set NPM_TOKEN") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
}

func TestDebugRun_BadArguments(t *testing.T) {
	s := twoSession(t)
	cases := map[string][]string{
		"from and step":        {"run", "two", "--from", "a", "--step", "b"},
		"with-needs alone":     {"run", "two", "--with-needs"},
		"with-needs and from":  {"run", "two", "--from", "a", "--with-needs"},
		"step without a value": {"run", "two", "--step"},
		"env without equals":   {"run", "two", "--env", "NOVALUE"},
		"env with a bad name":  {"run", "two", "--env", "1BAD=x"},
		"env with empty name":  {"run", "two", "--env", "=x"},
		"track with step":      {"run", "two", "--step", "a", "--track"},
		"track with from":      {"run", "two", "--from", "a", "--track"},
		"unknown step":         {"run", "two", "--step", "zzz"},
		"unknown from":         {"run", "two", "--from", "zzz"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := s.truss(args...)
			if code != 1 || strings.Contains(out, "marker-") {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
			}
			if errOut == "" {
				t.Error("no message")
			}
		})
	}
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "state", "two")); !os.IsNotExist(err) {
		t.Errorf("a refused debug run made a state folder: %v", err)
	}
	_, _, errOut := s.truss("run", "two", "--step", "zzz")
	for _, want := range []string{`"zzz"`, "a, b, c"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("unknown step message lacks %q: %s", want, errOut)
		}
	}
}

func TestDebugRun_ExitCodeOfAFailedStep(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/bad.yaml", "version: \"1\"\nsteps:\n  - {id: ok, run: \"echo fine\"}\n  - id: bad\n    needs: [ok]\n    run: '"+helperCmd("exit", "3")+"'\n")
	code, _, errOut := s.truss("run", "bad", "--step", "bad")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if strings.Contains(errOut, "to run only this step again") {
		t.Errorf("a debug run printed the hint:\n%s", errOut)
	}
}

func TestRun_FailureHintNamesTheStep(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", projectFile)
	s.write(".truss/flows/bad.yaml", "version: \"1\"\nsteps:\n  - {id: ok, run: \"echo fine\"}\n  - id: bad\n    needs: [ok]\n    run: '"+helperCmd("exit", "3")+"'\n  - {id: later, run: \"echo later\", needs: [bad]}\n")
	code, out, errOut := s.truss("run", "bad")
	if code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(errOut, "to run only this step again: truss run bad --step bad") {
		t.Errorf("stderr lacks the hint:\n%s", errOut)
	}
	if strings.Contains(out, "to run only this step again") || strings.Contains(stateLog(t, s.dir, "bad"), "to run only") {
		t.Error("the hint reached the log or standard output")
	}
	// The hint repeats the flow as it was written.
	_, _, errOut = s.truss("run", ".truss/flows/bad.yaml")
	if !strings.Contains(errOut, "truss run .truss/flows/bad.yaml --step bad") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestRun_SuccessPrintsNoHint(t *testing.T) {
	s := twoSession(t)
	if _, _, errOut := s.truss("run", "two"); strings.Contains(errOut, "to run only") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestRun_AnyOtherArgumentIsStillThePath(t *testing.T) {
	// Only the four new words are options now (spec 023 Compatibility).
	s := twoSession(t)
	s.write("my flow.yaml", strings.Replace(twoSteps(), "name: two", "name: spaced", 1))
	if code, out, errOut := s.truss("run", "my flow.yaml"); code != 0 || !strings.Contains(out, "marker-c") {
		t.Fatalf("exit = %d: %s", code, errOut)
	}
}

func TestShellWord(t *testing.T) {
	for in, want := range map[string]string{
		"build/package":    "build/package",
		".truss/flows/a.y": ".truss/flows/a.y",
		"my flow.yaml":     `"my flow.yaml"`,
		`a"b`:              `"a\"b"`,
		"":                 `""`,
	} {
		if got := shellWord(in); got != want {
			t.Errorf("shellWord(%q) = %s, want %s", in, got, want)
		}
	}
}
