package pipeline

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFile_ValidDocument(t *testing.T) {
	p, err := ParseFile(context.Background(), filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if p.Version != "1" {
		t.Fatalf("Version = %q", p.Version)
	}
	if p.Name != "hello-world" {
		t.Fatalf("Name = %q", p.Name)
	}
	if len(p.Steps) != 2 || p.Steps[0].ID != "hello" || p.Steps[1].ID != "second" {
		t.Fatalf("Steps = %+v", p.Steps)
	}
	if p.Steps[0].Run != `echo "Hello from TRUSS"` {
		t.Fatalf("Run = %q", p.Steps[0].Run)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte(":\n  - ["))
	if err == nil || !strings.Contains(err.Error(), "invalid YAML") {
		t.Fatalf("error = %v", err)
	}
}

func TestParse_EmptyFile(t *testing.T) {
	_, err := Parse(nil)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestParse_Needs(t *testing.T) {
	const head = "version: \"1\"\npipeline:\n  name: n\nsteps:\n"
	tests := []struct {
		name string
		step string
		want []string
	}{
		{name: "omitted", step: "  - id: a\n    run: echo a\n", want: nil},
		{name: "empty list", step: "  - id: a\n    needs: []\n    run: echo a\n", want: nil},
		{name: "one id", step: "  - id: a\n    needs: [build]\n    run: echo a\n", want: []string{"build"}},
		{name: "several ids in list order", step: "  - id: a\n    needs: [lint, test]\n    run: echo a\n", want: []string{"lint", "test"}},
		{name: "entries are trimmed", step: "  - id: a\n    needs: [\" build \"]\n    run: echo a\n", want: []string{"build"}},
		// FR-016 replaced the old "unknown fields are ignored" rule: depends_on is
		// still not a dependency, but Validate now rejects it (see
		// TestParse_DependsOnIsNotNeeds).
		{name: "scalar needs does not fail parse", step: "  - id: a\n    needs: build\n    run: echo a\n", want: nil},
		{name: "number entry does not fail parse", step: "  - id: a\n    needs: [1]\n    run: echo a\n", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(head + tt.step))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if len(p.Steps) != 1 {
				t.Fatalf("Steps = %+v", p.Steps)
			}
			got := p.Steps[0].Needs
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("Needs = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFile_MissingPath(t *testing.T) {
	_, err := ParseFile(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
}

// FR-016 replaces the old rule that ignored unknown fields. Parse keeps no
// dependency from depends_on and does not fail; Validate rejects the field.
func TestParse_DependsOnIsNotNeeds(t *testing.T) {
	const doc = "version: \"1\"\npipeline:\n  name: n\nsteps:\n  - id: a\n    depends_on: [build]\n    run: echo a\n"
	p, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(p.Steps) != 1 || len(p.Steps[0].Needs) != 0 {
		t.Fatalf("Steps = %+v, want no needs", p.Steps)
	}
	err = Validate(p)
	if want := `unknown field "depends_on" in step "a"`; err == nil || err.Error() != want {
		t.Fatalf("Validate() error = %v, want %q", err, want)
	}
}

func TestParse_Concurrency(t *testing.T) {
	const tail = "steps:\n  - id: a\n    run: echo a\n"
	tests := []struct {
		name string
		doc  string
		want int
	}{
		{name: "declared 2", doc: "version: \"1\"\npipeline: {name: x, concurrency: 2}\n" + tail, want: 2},
		{name: "declared 1", doc: "version: \"1\"\npipeline: {name: x, concurrency: 1}\n" + tail, want: 1},
		{name: "declared 16", doc: "version: \"1\"\npipeline:\n  name: x\n  concurrency: 16\n" + tail, want: 16},
		{name: "omitted is 0, meaning not declared", doc: "version: \"1\"\npipeline:\n  name: x\n" + tail, want: 0},
		{name: "number too large for int clamps to the maximum", doc: "version: \"1\"\npipeline:\n  name: x\n  concurrency: 99999999999999999999\n" + tail, want: math.MaxInt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.doc))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if p.Concurrency != tt.want {
				t.Fatalf("Concurrency = %d, want %d", p.Concurrency, tt.want)
			}
			if err := Validate(p); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

// Schema faults are reported by Validate; Parse fails only on invalid YAML.
func TestParse_DoesNotFailOnSchemaFaults(t *testing.T) {
	docs := map[string]string{
		"invalid concurrency":    "version: \"1\"\npipeline:\n  name: x\n  concurrency: two\nsteps:\n  - id: a\n    run: echo a\n",
		"empty concurrency":      "version: \"1\"\npipeline:\n  name: x\n  concurrency:\nsteps:\n  - id: a\n    run: echo a\n",
		"list concurrency":       "version: \"1\"\npipeline:\n  name: x\n  concurrency: [1]\nsteps:\n  - id: a\n    run: echo a\n",
		"unknown top level":      "version: \"1\"\nstpes: []\npipeline:\n  name: x\nsteps:\n  - id: a\n    run: echo a\n",
		"unknown in pipeline":    "version: \"1\"\npipeline:\n  name: x\n  concurency: 2\nsteps:\n  - id: a\n    run: echo a\n",
		"unknown in step":        "version: \"1\"\npipeline:\n  name: x\nsteps:\n  - id: a\n    need: [b]\n    run: echo a\n",
		"unknown in unnamed one": "version: \"1\"\npipeline:\n  name: x\nsteps:\n  - need: [b]\n    run: echo a\n",
	}
	for name, doc := range docs {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err != nil {
				t.Fatalf("Parse() error = %v, want nil (Validate reports schema faults)", err)
			}
		})
	}
}
