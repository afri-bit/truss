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

func TestParseFile_MissingPath(t *testing.T) {
	_, err := ParseFile(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
}
