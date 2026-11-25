package main

import (
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

// Spec 026: a local run gets the built-in variables and the project env.
func TestBuiltins_ALocalRunGetsTheVariablesAndTheProjectEnv(t *testing.T) {
	s := newSession(t, false)
	s.write("truss.yaml", "version: \"1\"\nproject:\n  name: demo\nenv:\n  REGION: eu\n  BUCKET: data-${REGION}\n")
	s.write(".truss/flows/vars.yaml", "version: \"1\"\nflow: {name: vars}\nenv:\n  PATHS: ${BUCKET}/in\nsteps:\n  - id: show\n    run: "+
		printVar("TRUSS_RUN_ID")+" && "+printVar("TRUSS_TRIGGER")+" && "+printVar("TRUSS_PROJECT_DIR")+" && "+printVar("PATHS")+"\n")
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
