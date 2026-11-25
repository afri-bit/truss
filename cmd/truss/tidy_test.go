package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// Spec 025: a mistyped command gets a suggestion, not the whole usage.
func TestTidy_UnknownCommandSuggestsTheClosest(t *testing.T) {
	s := newSession(t, false)
	code, out, errOut := s.truss("regster", "x")
	want := "error: unknown command \"regster\"\nhint: did you mean \"register\"?\nusage: truss <command> [arguments]\nRun \"truss help\" to see every command.\n"
	if code != 1 || out != "" || errOut != want {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, _, errOut := s.truss("zzzzzz"); strings.Contains(errOut, "did you mean") {
		t.Fatalf("a far word got a suggestion: %q", errOut)
	}
}

func TestTidy_WrongArgumentsShowTheCommandsUsage(t *testing.T) {
	s := newSession(t, false)
	code, _, errOut := s.truss("daemon", "restart")
	if code != 1 || !strings.HasPrefix(errOut, "error: truss daemon has no \"restart\"; did you mean \"start\"?\nusage: truss daemon ") ||
		!strings.HasSuffix(errOut, "Run \"truss help daemon\" for details.\n") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	_, _, errOut = s.truss("cancel")
	if errOut != "error: wrong arguments for truss cancel\nusage: truss cancel <run-id>\nRun \"truss help cancel\" for details.\n" {
		t.Fatalf("cancel: %q", errOut)
	}
	// A command without a topic of its own shows the topic it shares.
	if _, _, errOut := s.truss("pause"); !strings.Contains(errOut, "usage: truss register") {
		t.Fatalf("pause: %q", errOut)
	}
}

func TestTidy_AdviceGoesOnAHintLine(t *testing.T) {
	for msg, want := range map[string][2]string{
		"the TRUSS daemon is not running (start it with: truss daemon start)": {"the TRUSS daemon is not running", "start it with: truss daemon start"},
		"no flow named \"x\" is registered (list them with: truss pipelines)": {"no flow named \"x\" is registered", "list them with: truss pipelines"},
		"a plain message":                    {"a plain message", ""},
		"a note (not advice)":                {"a note (not advice)", ""},
		"bad (x) value (fix it with: a (b))": {"bad (x) value (fix it with: a (b))", ""},
	} {
		text, hint := splitHint(msg)
		if text != want[0] || hint != want[1] {
			t.Errorf("splitHint(%q) = %q, %q", msg, text, hint)
		}
	}
	s := newSession(t, false)
	code, _, errOut := s.truss("runs")
	if code != 1 || errOut != "error: the TRUSS daemon is not running\nhint: start it with: truss daemon start\n" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var in invocation
	var b strings.Builder
	in.stderr = &b
	in.fail(errors.New("x (do it with: y)"))
	if b.String() != "error: x\nhint: do it with: y\n" {
		t.Fatalf("fail = %q", b.String())
	}
}

func TestTidy_RunsTakesNAsLimit(t *testing.T) {
	s := newSession(t, false)
	if _, _, errOut := s.truss("runs", "-n", "0"); !strings.Contains(errOut, "--limit must be a whole number") {
		t.Fatalf("-n 0: %q", errOut)
	}
	if _, _, errOut := s.truss("runs", "-n", "2", "--limit", "3"); !strings.Contains(errOut, "same option") {
		t.Fatalf("-n and --limit: %q", errOut)
	}
}

func TestTidy_CompletionScripts(t *testing.T) {
	s := newSession(t, false)
	for shell, marker := range map[string]string{
		"bash": "complete -o default -F _truss truss", "zsh": "compdef _truss truss",
		"fish": "complete -c truss", "powershell": "Register-ArgumentCompleter -Native -CommandName truss",
	} {
		out := s.ok("completion", shell)
		if !strings.Contains(out, marker) || !strings.Contains(out, "register") || !strings.Contains(out, "uninstall") {
			t.Errorf("completion %s lacks commands or %q:\n%s", shell, marker, out)
		}
	}
	if code, _, errOut := s.truss("completion", "tcsh"); code != 1 || !isUsageError(errOut) {
		t.Fatalf("tcsh: %d %q", code, errOut)
	}
	// The bash script is valid bash.
	if bash, err := exec.LookPath("bash"); err == nil {
		cmd := exec.Command(bash, "-n")
		cmd.Stdin = strings.NewReader(s.ok("completion", "bash"))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("bash -n: %v %s", err, out)
		}
	}
}

func TestTidy_EveryCommandIsKnown(t *testing.T) {
	for _, c := range commands {
		if c == "help" || c == "version" {
			continue
		}
		s := newSession(t, false)
		if _, _, errOut := s.truss(c, "--definitely-not-a-flag", "x", "y", "z"); strings.Contains(errOut, "unknown command") {
			t.Errorf("%s is in commands but RunWithInput does not know it", c)
		}
	}
}
