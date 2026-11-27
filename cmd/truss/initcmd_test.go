package main

import (
	"bufio"
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

// The questions, answered by a person: a schedule in plain words, a
// notification, then a file hook.
func TestInit_AnswersBecomeAValidFlow(t *testing.T) {
	dir := t.TempDir()
	for input, want := range map[string][]string{
		"nightly\n6\n2\n3\ny\n":                 {"name: nightly", `cron: "0 9 * * 1-5"`, "desktop: true", "echo \"Hello from TRUSS\""},
		"sorter\n6\n3\ninbox/\nn\n":             {"name: sorter", `paths: ["inbox/"]`},
		"weird\n6\n2\n5\nnope\n15 3 * * *\nn\n": {`cron: "15 3 * * *"`},
	} {
		var out strings.Builder
		a := &asker{in: bufio.NewReader(strings.NewReader(input)), out: &out}
		ans, err := a.askFlow(dir, "build", "")
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		text := flowFile(ans)
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("%q: flow lacks %q:\n%s", input, w, text)
			}
		}
		p, err := pipeline.Parse([]byte(text))
		if err == nil {
			err = pipeline.Validate(p)
		}
		if err != nil {
			t.Errorf("%q: the flow does not validate: %v\n%s", input, err, text)
		}
	}
}
