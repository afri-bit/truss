package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodMeta = "version: \"1\"\nproject:\n  name: demo\n  description: A demo\n"

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRoot makes a project folder with a valid truss.yaml and the given flow files.
func newRoot(t *testing.T, flows ...string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "truss.yaml"), goodMeta)
	for _, f := range flows {
		write(t, filepath.Join(root, ".truss", "flows", f), "version: \"1\"\n")
	}
	return root
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
}

func TestOpen_ReadsMetadata(t *testing.T) {
	root := newRoot(t)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "demo" || p.Description != "A demo" || p.Root != root {
		t.Fatalf("project = %+v", p)
	}
}

func TestOpen_DescriptionIsOptional(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "truss.yaml"), "version: \"1\"\nproject:\n  name: demo\n")
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_NoProjectNamesTheFolder(t *testing.T) {
	root := t.TempDir()
	_, err := Open(root)
	if !errors.Is(err, ErrNoProject) || !strings.Contains(err.Error(), root) {
		t.Fatalf("err = %v", err)
	}
}

func TestOpen_DoesNotLookInParents(t *testing.T) {
	root := newRoot(t)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(sub); !errors.Is(err, ErrNoProject) {
		t.Fatalf("err = %v, want ErrNoProject", err)
	}
}

func TestOpen_RejectsBadMetadata(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"steps":           {"version: \"1\"\nproject: {name: a}\nsteps: []\n", ".truss/flows"},
		"jobs":            {"version: \"1\"\nproject: {name: a}\njobs: {}\n", ".truss/flows"},
		"unknown top":     {"version: \"1\"\nproject: {name: a}\nextra: SECRETVALUE\n", `unknown field "extra" at top level`},
		"unknown project": {"version: \"1\"\nproject: {name: a, owner: SECRETVALUE}\n", `unknown field "owner" in project`},
		"no version":      {"project: {name: a}\n", "version"},
		"bad version":     {"version: \"2\"\nproject: {name: a}\n", "version"},
		"int version":     {"version: 1\nproject: {name: a}\n", "version"},
		"no project":      {"version: \"1\"\n", "project.name"},
		"no name":         {"version: \"1\"\nproject: {description: x}\n", "project.name"},
		"bad name":        {"version: \"1\"\nproject: {name: \"-SECRETVALUE\"}\n", "project.name"},
		"empty":           {"", "version"},
		"not a mapping":   {"- a\n", "mapping"},
		"project string":  {"version: \"1\"\nproject: SECRETVALUE\n", "project"},
		"invalid yaml":    {"version: [\n", "invalid YAML"},
		"duplicate":       {"version: \"1\"\nproject: {name: a}\nversion: \"1\"\n", "duplicate"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "truss.yaml"), c.body)
			_, err := Open(root)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
			if strings.Contains(err.Error(), "SECRETVALUE") {
				t.Fatalf("error echoes a value: %v", err)
			}
		})
	}
}

func TestOpen_RefusesLinkAndHugeFile(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(t.TempDir(), "real.yaml")
	write(t, other, goodMeta)
	symlink(t, other, filepath.Join(root, "truss.yaml"))
	if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("err = %v, want a link error", err)
	}
	big := t.TempDir()
	write(t, filepath.Join(big, "truss.yaml"), goodMeta+"# "+strings.Repeat("x", MaxFileBytes))
	if _, err := Open(big); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v, want too large", err)
	}
}

func names(flows []Flow) string {
	var n []string
	for _, f := range flows {
		n = append(n, f.Name+"="+f.File)
	}
	return strings.Join(n, ",")
}

func TestFlows_ListsYamlAndYmlSorted(t *testing.T) {
	root := newRoot(t, "test.yml", "build.yaml", "a.b.yaml")
	p, _ := Open(root)
	flows, err := p.Flows()
	if err != nil {
		t.Fatal(err)
	}
	if got := names(flows); got != "a.b=a.b.yaml,build=build.yaml,test=test.yml" {
		t.Fatalf("flows = %s", got)
	}
	if flows[1].Path != filepath.Join(root, ".truss", "flows", "build.yaml") {
		t.Fatalf("path = %s", flows[1].Path)
	}
}

func TestFlows_IgnoresOtherEntries(t *testing.T) {
	root := newRoot(t, "ok.yaml", ".hidden.yaml", "notes.txt", "UP.YAML", "x.Yml", "noext")
	flowsDir := filepath.Join(root, ".truss", "flows")
	if err := os.Mkdir(filepath.Join(flowsDir, "dir.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, filepath.Join(root, "truss.yaml"), filepath.Join(flowsDir, "link.yaml"))
	p, _ := Open(root)
	flows, err := p.Flows()
	if err != nil {
		t.Fatal(err)
	}
	if got := names(flows); got != "ok=ok.yaml" {
		t.Fatalf("flows = %s", got)
	}
}

func TestFlows_MissingFoldersAreNoFlows(t *testing.T) {
	root := newRoot(t)
	p, _ := Open(root)
	if flows, err := p.Flows(); err != nil || len(flows) != 0 {
		t.Fatalf("flows = %v, err = %v", flows, err)
	}
	if err := os.Mkdir(filepath.Join(root, ".truss"), 0o755); err != nil {
		t.Fatal(err)
	}
	if flows, err := p.Flows(); err != nil || len(flows) != 0 {
		t.Fatalf("flows = %v, err = %v", flows, err)
	}
}

func TestFlows_LinkedOrFileFoldersAreErrors(t *testing.T) {
	t.Run(".truss link", func(t *testing.T) {
		root := newRoot(t)
		symlink(t, t.TempDir(), filepath.Join(root, ".truss"))
		p, _ := Open(root)
		if _, err := p.Flows(); err == nil || !strings.Contains(err.Error(), "link") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("flows link", func(t *testing.T) {
		root := newRoot(t)
		if err := os.Mkdir(filepath.Join(root, ".truss"), 0o755); err != nil {
			t.Fatal(err)
		}
		symlink(t, t.TempDir(), filepath.Join(root, ".truss", "flows"))
		p, _ := Open(root)
		if _, err := p.Flows(); err == nil || !strings.Contains(err.Error(), "link") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("flows is a file", func(t *testing.T) {
		root := newRoot(t)
		write(t, filepath.Join(root, ".truss", "flows"), "x")
		p, _ := Open(root)
		if _, err := p.Flows(); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestFlows_ClashesNameBothFiles(t *testing.T) {
	cases := map[string][2]string{
		"extension": {"build.yaml", "build.yml"},
		"case":      {"Build.yaml", "build.yaml"},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			root := newRoot(t, files[0], files[1])
			if _, err := os.Stat(filepath.Join(root, ".truss", "flows", files[0])); err != nil {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(filepath.Join(root, ".truss", "flows"))
			if len(entries) != 2 {
				t.Skip("case-insensitive file system")
			}
			p, _ := Open(root)
			_, err := p.Flows()
			if err == nil || !strings.Contains(err.Error(), files[0]) || !strings.Contains(err.Error(), files[1]) {
				t.Fatalf("err = %v, want both file names", err)
			}
		})
	}
}

func TestFlows_BadStemsAreErrors(t *testing.T) {
	for _, f := range []string{"-lead.yaml", "has space.yaml", "CON.yaml", "con.yml", "Nul.yaml", "COM1.yaml", "lpt9.yaml", "aux.build.yaml"} {
		t.Run(f, func(t *testing.T) {
			root := newRoot(t, f)
			p, _ := Open(root)
			if _, err := p.Flows(); err == nil || !strings.Contains(err.Error(), f) {
				t.Fatalf("err = %v, want it to name %s", err, f)
			}
		})
	}
	// COM0 and COMMON are fine.
	root := newRoot(t, "COM0.yaml", "common.yaml")
	p, _ := Open(root)
	if _, err := p.Flows(); err != nil {
		t.Fatal(err)
	}
}

func TestFlows_MoreThanTheCapIsAnError(t *testing.T) {
	root := newRoot(t)
	dir := filepath.Join(root, ".truss", "flows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range MaxFlowEntries {
		write(t, filepath.Join(dir, "f"+itoa(i)+".yaml"), "x")
	}
	p, _ := Open(root)
	if flows, err := p.Flows(); err != nil || len(flows) != MaxFlowEntries {
		t.Fatalf("at the cap: %d flows, err = %v", len(flows), err)
	}
	write(t, filepath.Join(dir, "one-more.yaml"), "x")
	if _, err := p.Flows(); err == nil || !strings.Contains(err.Error(), "256") {
		t.Fatalf("err = %v", err)
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	s := ""
	for ; i > 0; i /= 10 {
		s = string(digits[i%10]) + s
	}
	return s
}

func TestResolve(t *testing.T) {
	root := newRoot(t, "build.yaml", "test.yml")
	p, _ := Open(root)
	f, err := p.Resolve("test")
	if err != nil || f.File != "test.yml" {
		t.Fatalf("flow = %+v, err = %v", f, err)
	}
	_, err = p.Resolve("deploy")
	var nf *NotFoundError
	if !errors.As(err, &nf) || !strings.Contains(err.Error(), "build") || !strings.Contains(err.Error(), "test") {
		t.Fatalf("err = %v, want not found listing build and test", err)
	}
	// Case matters.
	if _, err := p.Resolve("Build"); err == nil {
		t.Fatal("Build resolved to build")
	}
}

func TestResolve_NoFlowsSaysSo(t *testing.T) {
	p, _ := Open(newRoot(t))
	_, err := p.Resolve("x")
	if err == nil || !strings.Contains(err.Error(), "no pipelines") {
		t.Fatalf("err = %v", err)
	}
}

func TestIsPath(t *testing.T) {
	for arg, want := range map[string]bool{
		"build": false, "build.yaml": true, "a/b": true, `a\b`: true, "x.yml": true,
		"./x": true, "name.v2": false, "X.YAML": false,
	} {
		if got := IsPath(arg); got != want {
			t.Errorf("IsPath(%q) = %v, want %v", arg, got, want)
		}
	}
}

func TestFileStem(t *testing.T) {
	for path, want := range map[string]string{
		"/a/build.yaml": "build", "x.yml": "x", "/a/b.c.yaml": "b.c", "/a/noext": "noext", "/a/x.txt": "x",
	} {
		if got := FileStem(path); got != want {
			t.Errorf("FileStem(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestStateFor(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "r")
	s, err := StateFor(root, "build")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".truss", "state", "build")
	if s.Truss != filepath.Join(root, ".truss") || s.StateDir != filepath.Join(root, ".truss", "state") || s.Dir != want ||
		s.Log != filepath.Join(want, "run.log") || s.Data != filepath.Join(want, "data") || s.Cache != filepath.Join(want, "data", "cache") {
		t.Fatalf("state = %+v", s)
	}
	for _, bad := range []string{"", ".", "..", "a.", "a ", "con", "NUL", "a/b", `a\b`, "a:b", "a\x00b"} {
		if _, err := StateFor(root, bad); err == nil {
			t.Errorf("StateFor(%q) succeeded", bad)
		}
	}
	if _, err := StateFor(root, "my pipeline"); err != nil {
		t.Errorf("a space inside a stem is fine: %v", err)
	}
}

func TestRootOf(t *testing.T) {
	root := newRoot(t, "build.yaml")
	file := filepath.Join(root, ".truss", "flows", "build.yaml")
	if got, ok := RootOf(file); !ok || got != root {
		t.Fatalf("RootOf = %q, %v", got, ok)
	}
	other := filepath.Join(root, "build.yaml")
	write(t, other, "x")
	if _, ok := RootOf(other); ok {
		t.Fatal("a file outside .truss/flows has a root")
	}
	if _, ok := RootOf(filepath.Join(".truss", "flows", "build.yaml")); ok {
		t.Fatal("a relative path has a root")
	}
	deep := filepath.Join(root, ".truss", "flows", "sub", "x.yaml")
	write(t, deep, "x")
	if _, ok := RootOf(deep); ok {
		t.Fatal("a file in a subfolder of flows has a root")
	}
}

func TestRootOf_NeedsTrussYAMLAndNoLinks(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".truss", "flows", "a.yaml"), "x")
	if _, ok := RootOf(filepath.Join(root, ".truss", "flows", "a.yaml")); ok {
		t.Fatal("root without truss.yaml")
	}
	// truss.yaml as a link does not count.
	target := filepath.Join(t.TempDir(), "t.yaml")
	write(t, target, goodMeta)
	if err := os.Symlink(target, filepath.Join(root, "truss.yaml")); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if _, ok := RootOf(filepath.Join(root, ".truss", "flows", "a.yaml")); ok {
		t.Fatal("a linked truss.yaml marks a project")
	}
	// A linked flows folder does not count.
	root2 := t.TempDir()
	write(t, filepath.Join(root2, "truss.yaml"), goodMeta)
	real := t.TempDir()
	write(t, filepath.Join(real, "a.yaml"), "x")
	if err := os.Mkdir(filepath.Join(root2, ".truss"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlink(t, real, filepath.Join(root2, ".truss", "flows"))
	if _, ok := RootOf(filepath.Join(root2, ".truss", "flows", "a.yaml")); ok {
		t.Fatal("a linked flows folder marks a project")
	}
}
