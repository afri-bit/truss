package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestMatchPattern(t *testing.T) {
	for _, c := range []struct {
		pattern, rel string
		want         bool
	}{
		{"inbox/", "inbox/a.jpg", true},
		{"inbox", "inbox/sub/a.jpg", true},
		{"inbox", "inboxes/a.jpg", false},
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "src/a/b/c.txt", false},
		{"**/*.tmp", "x/y.tmp", true},
		{"**/*.tmp", "y.tmp", true},
		{".git/**", ".git/HEAD", true},
		{"*.yaml", "a.yaml", true},
		{"*.yaml", "d/a.yaml", false},
		{"./notes.txt", "notes.txt", true},
	} {
		if got := MatchPattern(c.pattern, c.rel); got != c.want {
			t.Errorf("MatchPattern(%q, %q) = %v", c.pattern, c.rel, got)
		}
	}
	if PatternBase("src/**/*.go") != "src" || PatternBase("inbox/") != "inbox" || PatternBase("*.go") != "" {
		t.Error("PatternBase")
	}
	if CheckPattern("a/[b") == nil || CheckPattern(" ") == nil || CheckPattern("src/**/*.go") != nil {
		t.Error("CheckPattern")
	}
}

func TestParse_Hooks(t *testing.T) {
	src := "version: \"1\"\nflow: {name: f}\non:\n  file:\n    paths: [inbox/, \"src/**/*.go\"]\n    ignore: [\"**/*.tmp\"]\n    debounce: 5s\n" +
		"  run:\n    - flow: build\n      status: [succeeded, failed]\n    - flow: lint\nsteps:\n  - id: a\n    run: echo a\n"
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err != nil {
		t.Fatal(err)
	}
	if p.FileHook == nil || len(p.FileHook.Paths) != 2 || p.FileHook.Debounce != 5*time.Second || len(p.FileHook.Ignore) != 1 {
		t.Fatalf("file hook = %+v", p.FileHook)
	}
	if len(p.RunHooks) != 2 || !p.RunHooks[0].Matches("build", "failed") || p.RunHooks[1].Matches("lint", "failed") || !p.RunHooks[1].Matches("lint", "succeeded") {
		t.Fatalf("run hooks = %+v", p.RunHooks)
	}
	// A single mapping is one hook.
	p, _ = Parse([]byte("version: \"1\"\nflow: {name: f}\non:\n  run: {flow: build, status: any}\nsteps:\n  - id: a\n    run: echo a\n"))
	if err := Validate(p); err != nil || len(p.RunHooks) != 1 || !p.RunHooks[0].Matches("build", "cancelled") {
		t.Fatalf("single: %+v %v", p.RunHooks, err)
	}
}

func TestValidate_BadHooks(t *testing.T) {
	for src, want := range map[string]string{
		"on:\n  file: {}\n":                         "on.file must list one or more paths",
		"on:\n  file: {paths: [\"a/[b\"]}\n":        "path pattern",
		"on:\n  file: {paths: [a], debounce: 1h}\n": "debounce must be at most",
		"on:\n  file: {paths: [a], wait: 1s}\n":     `unknown field "wait" in on.file`,
		"on:\n  run: {status: failed}\n":            "on.run must name a flow",
		"on:\n  run: {flow: b, status: done}\n":     "status must be one of",
	} {
		p, err := Parse([]byte("version: \"1\"\nflow: {name: f}\n" + src + "steps:\n  - id: a\n    run: echo a\n"))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(p); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", src, err, want)
		}
	}
}
