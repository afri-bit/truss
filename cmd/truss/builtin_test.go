package main

import (
	"context"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func printVar(name string) string {
	if runtime.GOOS == "windows" {
		return "echo %" + name + "%"
	}
	return "echo $" + name
}

// and joins commands for the host shell; cmd.exe keeps a space before && in
// echo output, so there is none.
func and(cmds ...string) string {
	if runtime.GOOS == "windows" {
		return strings.Join(cmds, "&& ")
	}
	return strings.Join(cmds, " && ")
}

// Spec 026: a local run gets the built-in variables and the project env.
func TestBuiltins_ALocalRunGetsTheVariablesAndTheProjectEnv(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", "version: \"1\"\nproject:\n  name: demo\nenv:\n  REGION: eu\n  BUCKET: data-${REGION}\n")
	s.write(".truss/flows/vars.yaml", "version: \"1\"\nflow: {name: vars}\nenv:\n  PATHS: ${BUCKET}/in\nsteps:\n  - id: show\n    run: "+
		and(printVar("TRUSS_RUN_ID"), printVar("TRUSS_TRIGGER"), printVar("TRUSS_PROJECT_DIR"), printVar("PATHS"))+"\n")
	out := s.ok("run", "vars")
	if !regexp.MustCompile(`step show \| local-\d{8}-\d{6}-[0-9a-f]{6}\n`).MatchString(out) {
		t.Errorf("no local run id:\n%s", out)
	}
	if !strings.Contains(out, "step show | local\n") || !strings.Contains(out, "step show | data-eu/in\n") {
		t.Errorf("trigger or project env missing:\n%s", out)
	}
	root, _ := filepath.Abs(s.dir)
	if !strings.Contains(out, "step show | "+root+"\n") {
		t.Errorf("TRUSS_PROJECT_DIR is not %s:\n%s", root, out)
	}
}

func TestBuiltins_AFileMayNotSetABuiltInVariable(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nenv:\n  TRUSS_RUN_ID: mine\nsteps:\n  - id: a\n    run: echo a\n")
	code, _, errOut := s.truss("validate", "f.yaml")
	if code != 1 || !strings.Contains(errOut, "TRUSS_RUN_ID is set by TRUSS in every run") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	s.write("truss.yaml", "version: \"1\"\nproject:\n  name: demo\nenv:\n  TRUSS_STEP: x\n")
	if code, _, errOut := s.truss("project"); code != 1 || !strings.Contains(errOut, "TRUSS_STEP is set by TRUSS") {
		t.Fatalf("project env: exit %d, stderr %q", code, errOut)
	}
}

func TestBuiltins_ABadReferenceIsAValidationError(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nsteps:\n  - id: a\n    run: echo a\n    dir: ${OOPS\n")
	if code, _, errOut := s.truss("validate", "f.yaml"); code != 1 || !strings.Contains(errOut, "has no closing }") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestBuiltins_EnvOptionValuesAreTakenAsWritten(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nsteps:\n  - id: a\n    run: "+printVar("PRICE")+"\n")
	out := s.ok("run", "f.yaml", "--env", "PRICE=5$$ and ${HOME}")
	if !strings.Contains(out, "step a | 5$$ and ${HOME}\n") {
		t.Fatalf("--env value changed:\n%s", out)
	}
}

// Run from another folder, a project flow still gets its project's root and
// env (review of spec 026).
func TestBuiltins_ProjectDirIsTheFlowsProjectFromAnywhere(t *testing.T) {
	s := newSession(t, false)
	s.write("proj/truss.yaml", "version: \"1\"\nproject:\n  name: demo\nenv:\n  REGION: eu\n")
	s.write("proj/.truss/flows/v.yaml", "version: \"1\"\nflow: {name: v}\nsteps:\n  - id: a\n    run: "+
		and(printVar("TRUSS_PROJECT_DIR"), printVar("REGION"))+"\n")
	s.write("elsewhere/keep", "")
	root, _ := filepath.Abs(filepath.Join(s.dir, "proj"))
	other := filepath.Join(s.dir, "elsewhere")
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{"run", filepath.Join(root, ".truss", "flows", "v.yaml")}, other, s.env, &stdout, &stderr)
	out, errOut := stdout.String(), stderr.String()
	if code != 0 || !strings.Contains(out, "step a | "+root+"\n") || !strings.Contains(out, "step a | eu\n") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestBuiltins_ASecretMayNotUseABuiltInName(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nsecrets: [TRUSS_RUN_ID]\nsteps:\n  - id: a\n    run: echo a\n")
	if code, _, errOut := s.truss("validate", "f.yaml"); code != 1 || !strings.Contains(errOut, "cannot be a secret name") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

// The step view shows the directory a run would use.
func TestBuiltins_TheStepViewExpandsTheDir(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", "version: \"1\"\nproject:\n  name: demo\nenv:\n  INBOX: ${TRUSS_PROJECT_DIR}/in\n")
	s.write("in/keep", "")
	s.write(".truss/flows/v.yaml", "version: \"1\"\nflow: {name: v}\nsteps:\n  - id: a\n    run: echo a\n    dir: ${INBOX}\n  - id: b\n    run: echo b\n    dir: ${TRUSS_TEMP_DIR}\n")
	type step struct {
		RunDir   string `json:"run_dir"`
		DirError string `json:"dir_error"`
	}
	a := decode[struct{ Step step }](t, s.ok("flows", "show", "v", "a", "--json"))
	root, _ := filepath.Abs(s.dir)
	if a.Step.RunDir != filepath.Join(root, "in") {
		t.Fatalf("a: %+v", a.Step)
	}
	b := decode[struct{ Step step }](t, s.ok("flows", "show", "v", "b", "--json"))
	if !strings.Contains(b.Step.DirError, "TRUSS_TEMP_DIR, which each run sets") {
		t.Fatalf("b: %+v", b.Step)
	}
}
