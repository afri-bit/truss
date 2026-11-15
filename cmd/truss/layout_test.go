package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Spec 016: the project layout. Tests build a project folder with a root
// truss.yaml and pipelines in .truss/flows/.

const projectFile = "version: \"1\"\nproject:\n  name: demo\n"

// newProject returns a project folder with the given flows (file name -> text).
func newProject(t *testing.T, flows map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "truss.yaml"), projectFile)
	for name, body := range flows {
		writeFile(t, filepath.Join(dir, ".truss", "flows", name), body)
	}
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stateLog(t *testing.T, dir, flow string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".truss", "state", flow, "run.log"))
	if err != nil {
		t.Fatalf("read run log of %s: %v", flow, err)
	}
	return string(data)
}

func TestLayout_RunByNameRunsTheNamedFlow(t *testing.T) {
	dir := newProject(t, map[string]string{
		"build.yaml": validPipeline(echoHello()),
		"test.yml":   strings.Replace(validPipeline(echoHello()), "hello-world", "other", 1),
	})
	code, out, errOut := invoke(t, dir, []string{"run", "test"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if stateLog(t, dir, "test") != out {
		t.Fatalf("saved log differs from stdout")
	}
	if _, err := os.Stat(filepath.Join(dir, ".truss", "state", "build")); !os.IsNotExist(err) {
		t.Fatalf("the other flow got a state folder: %v", err)
	}
	if code, _, errOut := invoke(t, dir, []string{"run", "build"}); code != 0 {
		t.Fatalf("build: exit = %d, stderr = %s", code, errOut)
	}
	stateLog(t, dir, "build")
}

// SC-002: nothing outside .truss/state and .truss/.gitignore is created.
func TestLayout_RunCreatesOnlyStateAndGitignore(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(echoHello())})
	if code, _, errOut := invoke(t, dir, []string{"run", "build"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	var got []string
	entries, _ := os.ReadDir(filepath.Join(dir, ".truss"))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != ".gitignore,flows,state" {
		t.Fatalf(".truss holds %v", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".truss", ".gitignore"))
	if err != nil || strings.TrimSpace(string(data)) != "state/" {
		t.Fatalf(".gitignore = %q, %v", data, err)
	}
	top, _ := os.ReadDir(dir)
	if len(top) != 2 {
		t.Fatalf("the project folder holds %d entries, want truss.yaml and .truss", len(top))
	}
}

func TestLayout_GitignoreIsNeverOverwritten(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(echoHello())})
	writeFile(t, filepath.Join(dir, ".truss", ".gitignore"), "mine\n")
	if code, _, errOut := invoke(t, dir, []string{"run", "build"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".truss", ".gitignore")); string(got) != "mine\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

func TestLayout_GitignoreFailureWarnsAndContinues(t *testing.T) {
	var stderr bytes.Buffer
	writeGitignore(filepath.Join(t.TempDir(), "missing", ".gitignore"), &stderr)
	if !strings.HasPrefix(stderr.String(), "warning: ") {
		t.Fatalf("stderr = %q, want a warning", stderr.String())
	}
}

func TestLayout_StateFoldersAreOwnerOnlyAndHoldData(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(yamlSingle(helperCmd("print-env", "TRUSS_DATA_DIR")))})
	code, out, errOut := invoke(t, dir, []string{"run", "build"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	data := filepath.Join(dir, ".truss", "state", "build", "data")
	if !strings.Contains(out, data) {
		t.Fatalf("TRUSS_DATA_DIR is not in the state folder:\n%s", out)
	}
	if runtime.GOOS == "windows" {
		return
	}
	for _, p := range []string{
		filepath.Join(dir, ".truss", "state"),
		filepath.Join(dir, ".truss", "state", "build"),
		data,
		filepath.Join(data, "cache"),
	} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s: mode %v, err %v; want 0700", p, info.Mode().Perm(), err)
		}
	}
}

func TestLayout_RunWithoutArgumentListsFlows(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline("x"), "test.yml": validPipeline("x")})
	code, out, errOut := invoke(t, dir, []string{"run"})
	if code != 1 || out != "" || !strings.Contains(errOut, "build") || !strings.Contains(errOut, "test") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
}

func TestLayout_UnknownNameListsFlowsAndNeverFallsBackToAFile(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline("x")})
	writeFile(t, filepath.Join(dir, "other"), validPipeline(echoHello())) // a file named like the argument
	code, out, errOut := invoke(t, dir, []string{"run", "other"})
	if code != 1 || out != "" || !strings.Contains(errOut, `"other"`) || !strings.Contains(errOut, "build") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
}

func TestLayout_PathArgumentIsNeverAFlowName(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(echoHello())})
	// "build.yaml" is a path (relative to the working directory), not a flow.
	code, _, errOut := invoke(t, dir, []string{"run", "build.yaml"})
	if code != 1 || !strings.Contains(errOut, "error:") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	code, _, errOut = invoke(t, dir, []string{"run", filepath.Join(".truss", "flows", "build.yaml")})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	stateLog(t, dir, "build")
}

func TestLayout_RunTrussYAMLSaysItIsMetadata(t *testing.T) {
	dir := newProject(t, nil)
	code, _, errOut := invoke(t, dir, []string{"run", "truss.yaml"})
	if code != 1 || !strings.Contains(errOut, "project metadata") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestLayout_NameNeedsAProjectAndNamesTheFolder(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"run", "build"}, {"run"}, {"validate"}, {"validate", "build"}, {"register", "--all"}} {
		code, _, errOut := invoke(t, dir, args)
		if code != 1 || !strings.Contains(errOut, dir) || !strings.Contains(errOut, "truss.yaml") {
			t.Fatalf("%v: exit = %d, stderr = %q", args, code, errOut)
		}
	}
}

// FR-016: a pipeline given by path keeps its state under the working directory,
// project or not.
func TestLayout_RunByPathWritesStateUnderTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "ci", "deploy.yml")
	writeFile(t, elsewhere, validPipeline(echoHello()))
	if code, _, errOut := invoke(t, dir, []string{"run", elsewhere}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	stateLog(t, dir, "deploy")
	if _, err := os.Stat(filepath.Join(filepath.Dir(elsewhere), ".truss")); !os.IsNotExist(err) {
		t.Fatalf("state written next to the file: %v", err)
	}
}

func TestLayout_RunWithoutNameInPipelineUsesTheStem(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": "version: \"1\"\nsteps:\n  - id: a\n    run: " + echoHello() + "\n"})
	code, out, errOut := invoke(t, dir, []string{"run", "build"})
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if !strings.Contains(out, "summary succeeded") {
		t.Fatalf("out = %s", out)
	}
}

// FR-014 and SC-003: no link is followed under .truss.
func TestLayout_LinksUnderTrussAreRefused(t *testing.T) {
	for _, linked := range []string{
		".truss",
		filepath.Join(".truss", "flows"),
		filepath.Join(".truss", "state"),
		filepath.Join(".truss", "state", "build"),
	} {
		t.Run(filepath.ToSlash(linked), func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "truss.yaml"), projectFile)
			victim := t.TempDir()
			pipeline := filepath.Join(t.TempDir(), "build.yaml")
			writeFile(t, pipeline, validPipeline(echoHello()))
			if err := os.MkdirAll(filepath.Join(dir, ".truss", "flows"), 0o755); err != nil && linked != ".truss" {
				t.Fatal(err)
			}
			link := filepath.Join(dir, linked)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			_ = os.RemoveAll(link)
			plantLink(t, victim, link)
			args := []string{"run", pipeline}
			if strings.HasSuffix(linked, "flows") {
				args = []string{"run", "build"}
			}
			code, _, errOut := invoke(t, dir, args)
			if code != 1 || !strings.Contains(errOut, "link") {
				t.Fatalf("exit = %d, stderr = %q; want 1 and an error naming the link", code, errOut)
			}
			if entries, _ := os.ReadDir(victim); len(entries) != 0 {
				t.Fatalf("files appeared behind the link: %v", entries)
			}
		})
	}
}

func TestLayout_PlantedGitignoreLinkIsNotWrittenThrough(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(echoHello())})
	victim := filepath.Join(t.TempDir(), "victim")
	writeFile(t, victim, "keep\n")
	plantLink(t, victim, filepath.Join(dir, ".truss", ".gitignore"))
	if code, _, errOut := invoke(t, dir, []string{"run", "build"}); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, errOut)
	}
	if got, _ := os.ReadFile(victim); string(got) != "keep\n" {
		t.Fatalf("victim = %q", got)
	}
}

func TestLayout_ValidateChecksTheProjectAndEveryFlow(t *testing.T) {
	dir := newProject(t, map[string]string{
		"build.yaml": validPipeline(echoHello()),
		"named.yaml": "version: \"1\"\npipeline:\n  name: other-name\nsteps:\n  - id: a\n    run: echo a\n",
	})
	code, out, errOut := invoke(t, dir, []string{"validate"})
	if code != 0 || errOut != "" {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	for _, want := range []string{
		"truss.yaml is valid: project demo",
		".truss/flows/build.yaml is valid: pipeline hello-world, 1 steps, concurrency 1",
		".truss/flows/named.yaml is valid: pipeline other-name, 1 steps, concurrency 1",
		`note: .truss/flows/named.yaml declares the pipeline name "other-name", not "named"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out)
		}
	}
}

func TestLayout_ValidateReportsABrokenFlowAndGoesOn(t *testing.T) {
	dir := newProject(t, map[string]string{
		"a-bad.yaml": "version: \"1\"\nsteps: []\n",
		"b-good.yml": validPipeline(echoHello()),
	})
	code, out, errOut := invoke(t, dir, []string{"validate"})
	if code != 1 || !strings.Contains(errOut, "a-bad.yaml") || !strings.Contains(out, "b-good.yml is valid") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
}

func TestLayout_ValidateWithNoFlowsFolderSaysSo(t *testing.T) {
	dir := newProject(t, nil)
	code, out, errOut := invoke(t, dir, []string{"validate"})
	if code != 0 || !strings.Contains(out, "truss.yaml is valid") || !strings.Contains(out, "no pipelines") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, out, errOut)
	}
}

func TestLayout_ValidateRejectsAPipelineInTrussYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "truss.yaml"), validPipeline(echoHello()))
	code, _, errOut := invoke(t, dir, []string{"validate"})
	if code != 1 || !strings.Contains(errOut, ".truss/flows/") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestLayout_ValidateOneByNameAndByPath(t *testing.T) {
	dir := newProject(t, map[string]string{"build.yaml": validPipeline(echoHello())})
	code, out, _ := invoke(t, dir, []string{"validate", "build"})
	if code != 0 || !strings.HasPrefix(out, ".truss/flows/build.yaml is valid: pipeline hello-world, 1 steps, concurrency 1\n") {
		t.Fatalf("by name: exit = %d, stdout = %q", code, out)
	}
	rel := filepath.Join(".truss", "flows", "build.yaml")
	code, out, _ = invoke(t, dir, []string{"validate", rel})
	if code != 0 || !strings.HasPrefix(out, rel+" is valid: pipeline hello-world") {
		t.Fatalf("by path: exit = %d, stdout = %q", code, out)
	}
	if code, _, errOut := invoke(t, dir, []string{"validate", "nope"}); code != 1 || !strings.Contains(errOut, "build") {
		t.Fatalf("unknown: exit = %d, stderr = %q", code, errOut)
	}
}

func TestLayout_ProjectMetadataErrorsNeverEchoValues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "truss.yaml"), "version: \"1\"\nproject: {name: a}\ntoken: hunter2\n")
	code, _, errOut := invoke(t, dir, []string{"validate"})
	if code != 1 || !strings.Contains(errOut, `"token"`) || strings.Contains(errOut, "hunter2") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
}

func TestLayout_RegisterByNamePathAndAll(t *testing.T) {
	s := newSession(t, true)
	flows := filepath.Join(s.dir, ".truss", "flows")
	writeFile(t, filepath.Join(s.dir, "truss.yaml"), projectFile)
	writeFile(t, filepath.Join(flows, "build.yaml"), validPipeline(echoHello()))
	writeFile(t, filepath.Join(flows, "test.yml"), "version: \"1\"\nsteps:\n  - id: a\n    run: "+echoHello()+"\n")

	if out := s.ok("register", "build"); !strings.Contains(out, "registered hello-world ") {
		t.Fatalf("register by name: %q", out)
	}
	if code, _, errOut := s.truss("register"); code != 1 || !strings.Contains(errOut, "build") || !strings.Contains(errOut, "test") {
		t.Fatalf("register with nothing: exit = %d, stderr = %q", code, errOut)
	}
	if code, _, errOut := s.truss("register", "--all", "--name", "x"); code != 1 || !strings.Contains(errOut, "--name") {
		t.Fatalf("--all with --name: exit = %d, stderr = %q", code, errOut)
	}
	out := s.ok("register", "--all")
	if !strings.Contains(out, "registered hello-world ") || !strings.Contains(out, "registered test ") {
		t.Fatalf("register --all: %q", out)
	}
	if out := s.ok("pipelines"); !strings.Contains(out, "hello-world") || !strings.Contains(out, "test") {
		t.Fatalf("pipelines: %q", out)
	}
}

func TestLayout_RegisterNameTakenByAnotherFileHintsAtName(t *testing.T) {
	s := newSession(t, true)
	writeFile(t, filepath.Join(s.dir, "truss.yaml"), projectFile)
	writeFile(t, filepath.Join(s.dir, ".truss", "flows", "a.yaml"), validPipeline(echoHello()))
	writeFile(t, filepath.Join(s.dir, ".truss", "flows", "b.yaml"), validPipeline(echoHello())) // same pipeline name
	s.ok("register", "a")
	code, _, errOut := s.truss("register", "b")
	if code != 1 || !strings.Contains(errOut, "--name") {
		t.Fatalf("exit = %d, stderr = %q", code, errOut)
	}
	if out := s.ok("register", "b", "--name", "second"); !strings.Contains(out, "registered second ") {
		t.Fatalf("out = %q", out)
	}
}
