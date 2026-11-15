package loginstart

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/")

// golden compares got with testdata/name; run "go test -update" to rewrite it.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("%s differs from the generated text\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

var (
	plain = Spec{Binary: "/usr/local/bin/truss", LogPath: "/home/me/.truss/daemon.log", User: `DESKTOP-1\me`}
	full  = Spec{
		Binary: "/home/me/my tools/truss", Address: "127.0.0.1:9000", MaxRuns: 2,
		Home: "/home/me/truss $HOME %h", LogPath: "/home/me/truss $HOME %h/daemon.log", User: `CORP\me & co`,
	}
)

func TestSystemdUnit(t *testing.T) {
	golden(t, "truss.service", SystemdUnit(plain))
	golden(t, "truss_full.service", SystemdUnit(full))
}

func TestLaunchdPlist(t *testing.T) {
	golden(t, "truss.plist", LaunchdPlist(plain))
	golden(t, "truss_full.plist", LaunchdPlist(full))
}

func TestTaskXML(t *testing.T) {
	doc, err := TaskXML(plain)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "truss-task.xml", doc)

	winFull := full
	winFull.Binary = `C:\Program Files\truss\truss.exe`
	doc, err = TaskXML(winFull)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "truss-task_full.xml", doc)
}

func TestTaskXML_Refusals(t *testing.T) {
	if _, err := TaskXML(Spec{Binary: "truss.exe"}); err == nil {
		t.Error("no user: want an error")
	}
	if _, err := TaskXML(Spec{Binary: `C:\%USERPROFILE%\truss.exe`, User: "me"}); err == nil {
		t.Error("a % in the binary path: want an error")
	}
}

func TestSchtasksArgs(t *testing.T) {
	task := TaskName(`DESKTOP-1\me`)
	lines := []string{
		"create: " + strings.Join(SchtasksCreateArgs(task, `C:\Users\me\.truss\login-task.xml`), " "),
		"query:  " + strings.Join(SchtasksQueryArgs(task), " "),
		"run:    " + strings.Join(SchtasksRunArgs(task), " "),
		"end:    " + strings.Join(SchtasksEndArgs(task), " "),
		"delete: " + strings.Join(SchtasksDeleteArgs(task), " "),
	}
	golden(t, "schtasks.args", strings.Join(lines, "\n")+"\n")
}

func TestEncodeUTF16(t *testing.T) {
	got := encodeUTF16("aé")
	want := []byte{0xFF, 0xFE, 'a', 0, 0xE9, 0}
	if string(got) != string(want) {
		t.Errorf("encodeUTF16 = % x, want % x", got, want)
	}
}

func TestSpecArgs(t *testing.T) {
	if got := strings.Join(plain.Args(), " "); got != "daemon run" {
		t.Errorf("plain args = %q", got)
	}
	if got := strings.Join(full.Args(), " "); got != "daemon run --address 127.0.0.1:9000 --max-runs 2" {
		t.Errorf("full args = %q", got)
	}
}

func TestInstall_RejectsControlCharacters(t *testing.T) {
	h := newFake("linux")
	if _, err := Install(t.Context(), h.host(), Spec{Binary: "/bin/tr\nuss"}, false); err == nil {
		t.Fatal("want an error for a newline in the binary path")
	}
	if len(h.files) != 0 || len(h.calls) != 0 {
		t.Errorf("nothing may be written or run: files=%v calls=%v", h.files, h.calls)
	}
}

func TestTemporaryReason(t *testing.T) {
	temp := []string{
		"/tmp/go-build123/b001/exe/truss",
		"/home/me/.cache/go-build/x/truss",
		"/tmp/truss",
		"/var/folders/ab/cd/T/truss",
		"/private/var/folders/ab/cd/T/truss",
		"/mnt/scratch/truss", // equals the tempDir passed below
		`C:\Users\Me\AppData\Local\Temp\go-build9\b001\exe\truss.exe`,
		`C:\Users\Me\AppData\Local\Temp\truss.exe`,
		`C:\Windows\Temp\truss.exe`,
	}
	for _, p := range temp {
		if TemporaryReason(p, "/mnt/scratch") == "" {
			t.Errorf("TemporaryReason(%q) = \"\", want a reason", p)
		}
	}
	fine := []string{
		"/usr/local/bin/truss",
		"/home/me/go/bin/truss",
		"/home/me/tmp-tools/truss",
		"/opt/tmpfiles/truss",
		`C:\Program Files\truss\truss.exe`,
		`C:\Users\Me\bin\truss.exe`,
		"/mnt/scratchpad/truss", // prefix of the tempDir but another directory
	}
	for _, p := range fine {
		if r := TemporaryReason(p, "/mnt/scratch"); r != "" {
			t.Errorf("TemporaryReason(%q) = %q, want \"\"", p, r)
		}
	}
}

func TestTaskName(t *testing.T) {
	for in, want := range map[string]string{
		`DESKTOP-1\me`: "truss-daemon-DESKTOP-1_me",
		"a b/c":        "truss-daemon-a_b_c",
		"":             "truss-daemon",
	} {
		if got := TaskName(in); got != want {
			t.Errorf("TaskName(%q) = %q, want %q", in, got, want)
		}
	}
	if TaskName(`PC\a`) == TaskName(`PC\b`) {
		t.Error("two users must not share a task")
	}
}

func TestWindowsArg(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:      `plain`,
		`two words`:  `"two words"`,
		`say "hi"`:   `"say \"hi\""`,
		`a\"b`:       `"a\\\"b"`,
		`dir with \`: `"dir with \\"`,
		``:           `""`,
	} {
		if got := windowsArg(in); got != want {
			t.Errorf("windowsArg(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestInstall_RejectsInvalidUTF8(t *testing.T) {
	f := newFake("darwin")
	if _, err := Install(t.Context(), f.host(), Spec{Binary: "/bin/tr\xffuss"}, false); err == nil {
		t.Fatal("want an error")
	}
	if len(f.files) != 0 {
		t.Errorf("files = %v", f.files)
	}
}

func TestIsGoBuildDir(t *testing.T) {
	for name, want := range map[string]bool{"go-build": true, "go-build123": true, "go-builds": false, "go-build-x": false, "build": false} {
		if got := isGoBuildDir(name); got != want {
			t.Errorf("isGoBuildDir(%q) = %v", name, got)
		}
	}
}
