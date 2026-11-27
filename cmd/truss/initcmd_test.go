package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/afri-bit/truss/internal/pipeline"
)

// Spec 031: truss init without a terminal takes every suggestion, writes a
// project and a flow that validate, and never overwrites.
func TestInit_TakesTheSuggestionsAndValidates(t *testing.T) {
	s := newSession(t, false)
	s.write("go.mod", "module x\n")
	out := s.ok("init")
	if !strings.Contains(out, "created truss.yaml") || !strings.Contains(out, "created .truss/flows/build.yaml (Go module") {
		t.Fatalf("init:\n%s", out)
	}
	s.ok("validate")
	data, _ := os.ReadFile(filepath.Join(s.dir, ".truss", "flows", "build.yaml"))
	if !strings.Contains(string(data), "go test ./...") {
		t.Fatalf("flow:\n%s", data)
	}
	if code, _, errOut := s.truss("init"); code != 1 || !strings.Contains(errOut, "a TRUSS project already") {
		t.Fatalf("second init: %d %q", code, errOut)
	}
	out = s.ok("flows", "new", "nightly", "--template", "backup")
	if !strings.Contains(out, "created .truss/flows/nightly.yaml") {
		t.Fatalf("flows new:\n%s", out)
	}
	s.ok("validate")
	if code, _, errOut := s.truss("flows", "new", "nightly"); code != 1 || !strings.Contains(errOut, "exists already") {
		t.Fatalf("again: %d %q", code, errOut)
	}
	if code, _, errOut := s.truss("flows", "new", "x", "--template", "rust"); code != 1 || !strings.Contains(errOut, `no template "rust"`) {
		t.Fatalf("bad template: %d %q", code, errOut)
	}
}

func TestInit_FlowsNewNeedsAProject(t *testing.T) {
	s := newSession(t, false)
	if code, _, errOut := s.truss("flows", "new", "x"); code != 1 || !strings.Contains(errOut, "truss.yaml") {
		t.Fatalf("%d %q", code, errOut)
	}
}

// The questions, answered by a person: the template first, then the name,
// how it starts and whether to notify.
func TestInit_AnswersBecomeAValidFlow(t *testing.T) {
	dir := t.TempDir()
	for input, want := range map[string][]string{
		"6\nnightly\n2\n3\ny\n":                 {"name: nightly", `cron: "0 9 * * 1-5"`, "desktop: true", "echo Hello from TRUSS"},
		"shell\nsorter\n3\ninbox/\nn\n":         {"name: sorter", `paths: ["inbox/"]`},
		"6\nweird\n2\n5\nnope\n15 3 * * *\nn\n": {`cron: "15 3 * * *"`},
		"6\nw\n3\n../up\nsrc/\nn\n":             {`paths: ["src/"]`},
		"backup\n\n\n\n":                        {"name: backup", "TRUSS_DATA_DIR"},
	} {
		var out strings.Builder
		a := newLineAsker(context.Background(), strings.NewReader(input), &out)
		ans, err := a.askFlow(dir, "", "")
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		text := flowFile(ans)
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("%q: flow lacks %q:\n%s", input, w, text)
			}
		}
		assertValid(t, ans.name, text)
	}
}

// Every template, started each way, with or without a notification, is a
// valid flow.
func TestInit_EveryTemplateAndStartIsValid(t *testing.T) {
	dir := t.TempDir()
	for _, tmpl := range templates {
		for _, start := range []string{"hand", "schedule", "files"} {
			for _, notify := range []bool{false, true} {
				ans := flowAnswers{dir: dir, name: tmpl.name + "-x", tmpl: tmpl, start: start, cron: "0 2 * * *", paths: "src/", notifyOn: notify}
				assertValid(t, ans.name, flowFile(ans))
			}
		}
	}
}

func assertValid(t *testing.T, name, text string) {
	t.Helper()
	p, err := pipeline.Parse([]byte(text))
	if err == nil {
		p.Name = name
		err = pipeline.Validate(p)
	}
	if err != nil {
		t.Errorf("the flow does not validate: %v\n%s", err, text)
	}
}

// The suggested name follows the chosen template; the commands follow the
// files found.
func TestInit_NamesAndCommandsFollowTheProject(t *testing.T) {
	s := newSession(t, false)
	s.ok("init", "--template", "backup")
	if _, err := os.Stat(filepath.Join(s.dir, ".truss", "flows", "backup.yaml")); err != nil {
		t.Fatalf("--template backup did not name the flow backup: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	py, _ := findTemplate("python")
	node, _ := findTemplate("node")
	if !strings.Contains(py.steps(dir), "pip install .") || !strings.Contains(node.steps(dir), "npm install") {
		t.Fatalf("python: %s\nnode: %s", py.steps(dir), node.steps(dir))
	}
}

// A Ctrl-C during the questions writes nothing.
func TestInit_CtrlCWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r, w := io.Pipe()
	defer w.Close()
	var out strings.Builder
	a := newLineAsker(ctx, r, &out)
	cancel()
	if _, err := a.askFlow(t.TempDir(), "", ""); err != errStopped {
		t.Fatalf("err = %v, want stopped", err)
	}
}

// A failure part way leaves the folder as it was.
func TestInit_AFailureLeavesNoHalfProject(t *testing.T) {
	s := newSession(t, false)
	s.write(".truss/flows/hello.yaml", "x")
	if code, _, errOut := s.truss("init"); code != 1 || !strings.Contains(errOut, "exists already") {
		t.Fatalf("%d %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "truss.yaml")); !os.IsNotExist(err) {
		t.Fatalf("truss.yaml was left behind: %v", err)
	}
	if safeName("My Project") != "My-Project" || safeName("日本") != "project" || safeName(".hidden") != "hidden" {
		t.Error("safeName")
	}
}
