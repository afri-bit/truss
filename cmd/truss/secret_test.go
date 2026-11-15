package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// canary is a made-up value, never a real credential.
const canary = "canary-cli-secret-0001"

// trussIn runs one CLI invocation with the given standard input.
func (s *session) trussIn(stdin string, args ...string) (int, string, string) {
	s.t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunWithInput(context.Background(), args, s.dir, s.env, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func (s *session) setSecret(name, value string) {
	s.t.Helper()
	if code, out, errOut := s.trussIn(value, "secret", "set", name); code != 0 {
		s.t.Fatalf("secret set: exit %d\n%s\n%s", code, out, errOut)
	}
}

const secretPipelineFile = `version: "1"
pipeline:
  name: with-secret
steps:
  - id: show
    run: '%s'
    secrets: [TRUSS_TEST_TOKEN]
`

func secretPipelineText() string {
	return strings.Replace(secretPipelineFile, "%s", helperCmd("print-env", "TRUSS_TEST_TOKEN"), 1)
}

func TestSecretCommands_SetListRemoveNeverPrintValues(t *testing.T) {
	s := newSession(t, false)
	if code, out, _ := s.truss("secret", "list"); code != 0 || out != "" {
		t.Fatalf("empty list: %d %q", code, out)
	}
	// One trailing newline is dropped; the rest of the value is kept.
	code, out, errOut := s.trussIn(canary+"\n", "secret", "set", "B_TOKEN")
	if code != 0 || out != "secret B_TOKEN saved\n" || errOut != "" {
		t.Fatalf("set: %d %q %q", code, out, errOut)
	}
	s.setSecret("A_TOKEN", canary+"-2\r\n")
	list := s.ok("secret", "list")
	if list != "A_TOKEN\nB_TOKEN\n" || strings.Contains(list, canary) {
		t.Fatalf("list = %q", list)
	}
	if out := s.ok("secret", "remove", "A_TOKEN"); strings.Contains(out, canary) {
		t.Fatalf("remove printed a value: %q", out)
	}
	if got := s.ok("secret", "list"); got != "B_TOKEN\n" {
		t.Fatalf("list after remove = %q", got)
	}
	code, _, errOut = s.truss("secret", "remove", "A_TOKEN")
	if code != 1 || !strings.HasPrefix(errOut, "error: ") || strings.Contains(errOut, canary) {
		t.Fatalf("remove unknown: %d %q", code, errOut)
	}
}

func TestSecretSet_RejectsBadInput(t *testing.T) {
	s := newSession(t, false)
	cases := map[string]struct {
		stdin string
		args  []string
	}{
		"bad name":      {canary, []string{"secret", "set", "bad-name"}},
		"empty value":   {"\n", []string{"secret", "set", "OK"}},
		"no stdin":      {"", []string{"secret", "set", "OK"}},
		"value as arg":  {"", []string{"secret", "set", "OK", canary}},
		"unknown sub":   {"", []string{"secret", "show"}},
		"missing sub":   {"", []string{"secret"}},
		"list with arg": {"", []string{"secret", "list", "x"}},
	}
	for name, c := range cases {
		code, out, errOut := s.trussIn(c.stdin, c.args...)
		if code != 1 {
			t.Errorf("%s: exit %d", name, code)
		}
		if strings.Contains(out+errOut, canary) {
			t.Errorf("%s: output repeats the value: %q %q", name, out, errOut)
		}
	}
	if got := s.ok("secret", "list"); got != "" {
		t.Errorf("a rejected set stored something: %q", got)
	}
}

func TestSecretStoreFile_IsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not exist on Windows")
	}
	s := newSession(t, false)
	s.setSecret("TOKEN", canary)
	var path string
	for _, kv := range s.env {
		if v, ok := strings.CutPrefix(kv, "TRUSS_HOME="); ok {
			path = filepath.Join(v, "secrets.json")
		}
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("secrets file: %v, %v", info, err)
	}
}

func TestRunLocal_UsesTheStoreAndRedactsOutput(t *testing.T) {
	s := newSession(t, false)
	s.write("pipeline.yaml", secretPipelineText())
	s.setSecret("TRUSS_TEST_TOKEN", canary)

	out := s.ok("run", "pipeline.yaml")
	if strings.Contains(out, canary) || !strings.Contains(out, "step show | ***") {
		t.Fatalf("run log:\n%s", out)
	}
	saved, err := os.ReadFile(filepath.Join(s.dir, ".truss", "state", "pipeline", "run.log"))
	if err != nil || strings.Contains(string(saved), canary) || !strings.Contains(string(saved), "***") {
		t.Fatalf("saved log (%v):\n%s", err, saved)
	}
}

func TestRunLocal_MissingSecretFailsBeforeAnyStep(t *testing.T) {
	s := newSession(t, false)
	s.write("pipeline.yaml", secretPipelineText())
	code, out, errOut := s.truss("run", "pipeline.yaml")
	if code != 1 || !strings.Contains(errOut, "TRUSS_TEST_TOKEN") || !strings.Contains(errOut, "truss secret set") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if strings.Contains(out, "started") {
		t.Fatalf("a step started:\n%s", out)
	}
}

func TestValidate_ListsSecretNamesOnly(t *testing.T) {
	s := newSession(t, false)
	s.write("pipeline.yaml", secretPipelineText())
	s.setSecret("TRUSS_TEST_TOKEN", canary)
	out := s.ok("validate", "pipeline.yaml")
	if !strings.Contains(out, "uses secrets TRUSS_TEST_TOKEN") || strings.Contains(out, canary) {
		t.Fatalf("validate:\n%s", out)
	}
	// A value pasted where a name belongs is not echoed back.
	s.write("bad.yaml", "version: \"1\"\npipeline: {name: x}\nsecrets: [\""+canary+"!\"]\nsteps:\n  - {id: a, run: x}\n")
	code, _, errOut := s.truss("validate", "bad.yaml")
	if code != 1 || strings.Contains(errOut, canary) {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestDaemon_RunUsesTheStoreAndRedactsLogAndAPI(t *testing.T) {
	s := newSession(t, true)
	s.write("pipeline.yaml", secretPipelineText())
	s.ok("register", "pipeline.yaml")

	// Missing: the run fails before any step and says what to do.
	code, out, errOut := s.truss("trigger", "with-secret", "--follow")
	if code != 1 || !strings.Contains(out+errOut, "missing secrets: TRUSS_TEST_TOKEN") || strings.Contains(out, "started") {
		t.Fatalf("missing secret: exit %d\n%s\n%s", code, out, errOut)
	}

	// Stored after the daemon started: the next run picks it up.
	s.setSecret("TRUSS_TEST_TOKEN", canary)
	out = s.ok("trigger", "with-secret", "--follow")
	if strings.Contains(out, canary) || !strings.Contains(out, "step show | ***") {
		t.Fatalf("followed run:\n%s", out)
	}
	id := runID.FindAllString(s.ok("runs", "with-secret"), -1)[0]
	for _, text := range []string{s.ok("logs", id), s.ok("runs"), s.ok("pipelines")} {
		if strings.Contains(text, canary) {
			t.Fatalf("a value leaked through the API:\n%s", text)
		}
	}
}

func TestTrimLineEnding(t *testing.T) {
	for in, want := range map[string]string{"v\n": "v", "v\r\n": "v", "v\r": "v\r", "v": "v", "v\n\n": "v\n", "": ""} {
		if got := trimLineEnding(in); got != want {
			t.Errorf("trimLineEnding(%q) = %q, want %q", in, got, want)
		}
	}
}
