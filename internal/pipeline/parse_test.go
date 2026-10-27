package pipeline

import (
	"context"
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
		{name: "depends_on is ignored", step: "  - id: a\n    depends_on: [build]\n    run: echo a\n", want: nil},
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
