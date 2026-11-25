package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
		"fish": "complete -c truss", "powershell": "Register-ArgumentCompleter -Native -CommandName 'truss', 'truss.exe'",
	} {
		out := s.ok("completion", shell)
		if !strings.Contains(out, marker) || !strings.Contains(out, "register") || !strings.Contains(out, "uninstall") {
			t.Errorf("completion %s lacks commands or %q:\n%s", shell, marker, out)
		}
	}
	if code, _, errOut := s.truss("completion", "tcsh"); code != 1 || !isUsageError(errOut) {
		t.Fatalf("tcsh: %d %q", code, errOut)
	}
	// Each script parses in its shell, where that shell is installed (CI's
	// Windows runner has pwsh).
	for shell, check := range map[string][]string{
		"bash": {"bash", "-n"}, "zsh": {"zsh", "-n"}, "fish": {"fish", "--no-execute"},
		"powershell": {"pwsh", "-NoProfile", "-Command", "$null = [scriptblock]::Create([Console]::In.ReadToEnd())"},
	} {
		bin, err := exec.LookPath(check[0])
		if err != nil {
			t.Logf("%s not installed; the %s script is not parsed", check[0], shell)
			continue
		}
		cmd := exec.Command(bin, check[1:]...)
		cmd.Stdin = strings.NewReader(s.ok("completion", shell))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s does not parse the %s script: %v %s", check[0], shell, err, out)
		}
	}
}

// The bash script completes commands, subcommands and flow names (with
// spaces) of the current folder.
func TestTidy_BashCompletionCompletes(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs bash on a Unix system")
	}
	s := newSession(t, false)
	s.write(".truss/flows/build.yaml", "x")
	s.write(".truss/flows/my flow.yml", "x")
	s.write(".truss/flows/README.md", "x")
	script := s.ok("completion", "bash")
	for line, want := range map[string]string{
		"truss reg":     "register",
		"truss daemon ": "install logs run start status stop uninstall",
		"truss runs ":   "build my\\ flow show",
		"truss run ":    "build my\\ flow",
		"truss run b":   "build",
	} {
		words := strings.Fields(line)
		if strings.HasSuffix(line, " ") {
			words = append(words, "")
		}
		quoted := make([]string, len(words))
		for i, w := range words {
			quoted[i] = "'" + w + "'"
		}
		prog := script + "\nCOMP_WORDS=(" + strings.Join(quoted, " ") + "); COMP_CWORD=" + strconv.Itoa(len(words)-1) +
			"; _truss; printf '%s\\n' \"${COMPREPLY[@]}\" | sort"
		cmd := exec.Command(bash, "-c", prog)
		cmd.Dir = s.dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%q: %v %s", line, err, out)
		}
		if got := strings.Join(strings.Split(strings.TrimSpace(string(out)), "\n"), " "); got != want {
			t.Errorf("%q completes to %q, want %q", line, got, want)
		}
	}
}

// A flow file name is data: completing it runs nothing (security review of
// spec 025: compgen -W expands $(...) in its word list).
func TestTidy_BashCompletionRunsNothingFromAFileName(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs bash on a Unix system")
	}
	s := newSession(t, false)
	for _, name := range []string{"$(touch PWNED1)", "`touch PWNED2`", "${HOME}", "a;b", "it's"} {
		s.write(".truss/flows/"+name+".yaml", "x")
	}
	prog := s.ok("completion", "bash") + "\nCOMP_WORDS=(truss run ''); COMP_CWORD=2; _truss; printf '%s\\n' \"${COMPREPLY[@]}\""
	cmd := exec.Command(bash, "-c", prog)
	cmd.Dir = s.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	for _, marker := range []string{"PWNED1", "PWNED2"} {
		if _, err := os.Stat(filepath.Join(s.dir, marker)); err == nil {
			t.Fatalf("completion ran code from a file name: %s exists", marker)
		}
	}
	for _, want := range []string{`\$\(touch\ PWNED1\)`, `\$\{HOME\}`, `a\;b`, `it\'s`} {
		if !strings.Contains(string(out), want+"\n") {
			t.Errorf("completion lacks the quoted %s:\n%s", want, out)
		}
	}
}

func TestTidy_HelpAndVersionWithExtraWordsPrintTheUsage(t *testing.T) {
	s := newSession(t, false)
	for _, args := range [][]string{{"version", "x"}, {"help", "help"}, {"help", "version"}, {"help", "-x"}} {
		code, out, errOut := s.truss(args...)
		if args[1] == "-x" {
			if code != 1 || !strings.Contains(errOut, "unknown command") {
				t.Errorf("%v: %d %q", args, code, errOut)
			}
			continue
		}
		if code != 1 || out != "" || errOut != usageText {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
}

func TestTidy_RunErrorsGetAHintLineToo(t *testing.T) {
	s := newSession(t, false)
	s.write("f.yaml", "version: \"1\"\nflow: {name: f}\nsteps:\n  - id: a\n    run: echo a\n")
	code, _, errOut := s.truss("run", "f.yaml", "--track")
	if code != 1 || !strings.Contains(errOut, "the TRUSS daemon is not running\nhint: start it with: truss daemon start\n") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

// commands, the help topics and the dispatcher agree.
func TestTidy_CommandTableMatchesHelp(t *testing.T) {
	for topic := range helpTopics {
		if topic != "exit-codes" && !contains(commands, topic) {
			t.Errorf("help topic %q is not in commands", topic)
		}
	}
	for alias := range helpAliases {
		if !contains(commands, alias) {
			t.Errorf("help alias %q is not in commands", alias)
		}
	}
	for c := range subcommands {
		if !contains(commands, c) {
			t.Errorf("subcommands of %q, which is not in commands", c)
		}
	}
}

func TestTidy_EveryCommandIsKnown(t *testing.T) {
	for _, c := range commands {
		s := newSession(t, false)
		if _, _, errOut := s.truss(c, "--definitely-not-a-flag", "x", "y", "z"); strings.Contains(errOut, "unknown command") {
			t.Errorf("%s is in commands but RunWithInput does not know it", c)
		}
	}
}
