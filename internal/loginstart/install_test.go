package loginstart

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// fake records every command and keeps files in memory. failOn makes the
// first command whose line contains the text fail.
type fake struct {
	os     string
	files  map[string]string
	calls  []string
	failOn []string
	out    map[string]string // command line -> output
}

func newFake(goos string) *fake {
	return &fake{os: goos, files: map[string]string{}, out: map[string]string{}}
}

func (f *fake) host() Host {
	return Host{OS: f.os, UserHome: "/home/me", UID: 501, Scratch: "/home/me/.truss", Run: f, Files: f}
}

func (f *fake) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, line)
	for _, bad := range f.failOn {
		if strings.Contains(line, bad) {
			return []byte("boom"), errors.New("exit status 1")
		}
	}
	if out, ok := f.out[line]; ok {
		return []byte(out), nil
	}
	// A service is inactive and an agent is not loaded unless a test says so.
	if strings.Contains(line, "is-active") || strings.Contains(line, "launchctl print") {
		return nil, errors.New("exit status 3")
	}
	if f.os == "windows" && strings.HasPrefix(line, "schtasks.exe /Query") {
		return nil, errors.New("exit status 1: no such task")
	}
	return nil, nil
}

func (f *fake) Read(path string) ([]byte, error) {
	if s, ok := f.files[path]; ok {
		return []byte(s), nil
	}
	return nil, os.ErrNotExist
}

func (f *fake) Write(path string, data []byte, _ os.FileMode) error {
	f.files[path] = string(data)
	return nil
}

func (f *fake) Remove(path string) error { delete(f.files, path); return nil }

func (f *fake) wantCalls(t *testing.T, want ...string) {
	t.Helper()
	if got := strings.Join(f.calls, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("commands:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

const (
	unitFile  = "/home/me/.config/systemd/user/truss.service"
	plistFile = "/home/me/Library/LaunchAgents/io.github.afri-bit.truss.plist"
)

var spec = Spec{Binary: "/usr/local/bin/truss", LogPath: "/home/me/.truss/daemon.log", User: `PC\me`}

func TestInstall_Systemd(t *testing.T) {
	f := newFake("linux")
	res, err := Install(t.Context(), f.host(), spec, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.files[unitFile] != SystemdUnit(spec) {
		t.Errorf("unit file = %q", f.files[unitFile])
	}
	f.wantCalls(t,
		"systemctl --user daemon-reload",
		"systemctl --user enable truss.service",
		"systemctl --user is-active --quiet truss.service",
		"systemctl --user restart truss.service",
	)
	out := strings.Join(res.Messages, "\n")
	for _, want := range []string{"installed", unitFile, "loginctl enable-linger"} {
		if !strings.Contains(out, want) {
			t.Errorf("messages lack %q:\n%s", want, out)
		}
	}
}

func TestInstall_SystemdWhenADaemonAlreadyRuns(t *testing.T) {
	f := newFake("linux")
	res, err := Install(t.Context(), f.host(), spec, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "restart") || strings.Contains(c, "start ") {
			t.Errorf("must not start a second daemon: %s", c)
		}
	}
	if out := strings.Join(res.Messages, "\n"); !strings.Contains(out, "next login") {
		t.Errorf("messages = %s", out)
	}
}

func TestInstall_SystemdReinstallRestartsTheServiceItOwns(t *testing.T) {
	f := newFake("linux")
	f.files[unitFile] = SystemdUnit(Spec{Binary: "/old/truss"})
	f.out["systemctl --user is-active --quiet truss.service"] = "" // active: exit 0
	res, err := Install(t.Context(), f.host(), spec, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.files[unitFile], "/usr/local/bin/truss") {
		t.Errorf("unit not rewritten: %s", f.files[unitFile])
	}
	if !strings.Contains(strings.Join(f.calls, "\n"), "restart truss.service") {
		t.Errorf("an active service must restart to apply the change: %v", f.calls)
	}
	if !strings.Contains(res.Messages[0], "updated") {
		t.Errorf("messages = %v", res.Messages)
	}
}

func TestInstall_SystemdRefusesAForeignUnit(t *testing.T) {
	f := newFake("linux")
	f.files[unitFile] = "[Service]\nExecStart=/mine\n"
	if _, err := Install(t.Context(), f.host(), spec, true); err == nil || !strings.Contains(err.Error(), "not created by truss") {
		t.Fatalf("err = %v", err)
	}
	if f.files[unitFile] != "[Service]\nExecStart=/mine\n" || len(f.calls) != 0 {
		t.Errorf("a foreign file must be left alone: %q %v", f.files[unitFile], f.calls)
	}
}

func TestInstall_SystemdRollsBackWhenSystemctlFails(t *testing.T) {
	f := newFake("linux")
	f.failOn = []string{"enable"}
	_, err := Install(t.Context(), f.host(), spec, true)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the command output in it", err)
	}
	if _, ok := f.files[unitFile]; ok {
		t.Error("the new unit file must be removed again")
	}
}

func TestUninstall_Systemd(t *testing.T) {
	f := newFake("linux")
	f.files[unitFile] = SystemdUnit(spec)
	f.files["/home/me/.config/systemd/user/other.service"] = "keep"
	if _, err := Uninstall(t.Context(), f.host()); err != nil {
		t.Fatal(err)
	}
	f.wantCalls(t, "systemctl --user disable --now truss.service", "systemctl --user daemon-reload")
	if len(f.files) != 1 || f.files["/home/me/.config/systemd/user/other.service"] != "keep" {
		t.Errorf("only the truss unit may go: %v", f.files)
	}
}

func TestUninstall_NothingInstalledIsFine(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		f := newFake(goos)
		res, err := Uninstall(t.Context(), f.host())
		if err != nil || !strings.Contains(strings.Join(res.Messages, ""), "not installed") {
			t.Errorf("%s: res=%v err=%v", goos, res, err)
		}
		for _, c := range f.calls {
			if strings.Contains(c, "/Delete") || strings.Contains(c, "disable") || strings.Contains(c, "bootout") {
				t.Errorf("%s: nothing to remove, but ran %s", goos, c)
			}
		}
	}
}

func TestUninstall_RefusesForeignFiles(t *testing.T) {
	for goos, path := range map[string]string{"linux": unitFile, "darwin": plistFile} {
		f := newFake(goos)
		f.files[path] = "not ours"
		if _, err := Uninstall(t.Context(), f.host()); err == nil {
			t.Errorf("%s: want an error", goos)
		}
		if f.files[path] != "not ours" || len(f.calls) != 0 {
			t.Errorf("%s: a foreign file must be left alone: %v", goos, f.calls)
		}
	}
}

func TestUninstall_SystemdKeepsGoingWhenDisableFails(t *testing.T) {
	f := newFake("linux")
	f.files[unitFile] = SystemdUnit(spec)
	f.failOn = []string{"disable"}
	res, err := Uninstall(t.Context(), f.host())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.files[unitFile]; ok {
		t.Error("the unit file must still be removed")
	}
	if !strings.Contains(strings.Join(res.Messages, "\n"), "warning") {
		t.Errorf("the failure must be reported: %v", res.Messages)
	}
}

func TestInstall_Launchd(t *testing.T) {
	f := newFake("darwin")
	if _, err := Install(t.Context(), f.host(), spec, true); err != nil {
		t.Fatal(err)
	}
	if f.files[plistFile] != LaunchdPlist(spec) {
		t.Errorf("plist = %q", f.files[plistFile])
	}
	f.wantCalls(t,
		"launchctl print gui/501/io.github.afri-bit.truss",
		"launchctl bootstrap gui/501 "+plistFile,
	)
}

func TestInstall_LaunchdWhenADaemonAlreadyRuns(t *testing.T) {
	f := newFake("darwin")
	res, err := Install(t.Context(), f.host(), spec, false)
	if err != nil {
		t.Fatal(err)
	}
	f.wantCalls(t, "launchctl print gui/501/io.github.afri-bit.truss")
	if !strings.Contains(strings.Join(res.Messages, "\n"), "next login") {
		t.Errorf("messages = %v", res.Messages)
	}
}

func TestInstall_LaunchdReloadsALoadedAgent(t *testing.T) {
	f := newFake("darwin")
	f.files[plistFile] = LaunchdPlist(Spec{Binary: "/old"})
	f.out["launchctl print gui/501/io.github.afri-bit.truss"] = "loaded"
	if _, err := Install(t.Context(), f.host(), spec, false); err != nil {
		t.Fatal(err)
	}
	f.wantCalls(t,
		"launchctl print gui/501/io.github.afri-bit.truss",
		"launchctl bootout gui/501/io.github.afri-bit.truss",
		"launchctl bootstrap gui/501 "+plistFile,
	)
}

func TestInstall_LaunchdRollsBack(t *testing.T) {
	f := newFake("darwin")
	f.failOn = []string{"bootstrap"}
	if _, err := Install(t.Context(), f.host(), spec, true); err == nil {
		t.Fatal("want an error")
	}
	if _, ok := f.files[plistFile]; ok {
		t.Error("the new plist must be removed again")
	}
}

func TestUninstall_Launchd(t *testing.T) {
	f := newFake("darwin")
	f.files[plistFile] = LaunchdPlist(spec)
	f.failOn = []string{"bootout"} // not loaded: ignored
	if _, err := Uninstall(t.Context(), f.host()); err != nil {
		t.Fatal(err)
	}
	f.wantCalls(t, "launchctl bootout gui/501/io.github.afri-bit.truss")
	if len(f.files) != 0 {
		t.Errorf("files left: %v", f.files)
	}
}

const xmlScratch = "/home/me/.truss/login-task.xml"

func TestInstall_Task(t *testing.T) {
	f := newFake("windows")
	var xmlAtCreate string
	f.files = map[string]string{}
	h := f.host()
	h.Run = runnerFunc(func(line string) ([]byte, error) {
		if strings.HasPrefix(line, "schtasks.exe /Create") {
			xmlAtCreate = f.files[xmlScratch]
		}
		fields := strings.Fields(line)
		return f.Run(t.Context(), fields[0], fields[1:]...)
	})
	res, err := Install(t.Context(), h, spec, true)
	if err != nil {
		t.Fatal(err)
	}
	want := string(encodeUTF16(mustTaskXML(t, spec)))
	if xmlAtCreate != want {
		t.Errorf("the task XML given to schtasks differs from TaskXML:\n%q", xmlAtCreate)
	}
	if _, ok := f.files[xmlScratch]; ok {
		t.Error("the temporary XML file must be removed")
	}
	f.wantCalls(t,
		"schtasks.exe /Query /TN truss-daemon /XML",
		"schtasks.exe /Create /TN truss-daemon /XML "+xmlScratch+" /F",
		"schtasks.exe /Run /TN truss-daemon",
	)
	if !strings.Contains(res.Messages[0], "installed") {
		t.Errorf("messages = %v", res.Messages)
	}
}

type runnerFunc func(line string) ([]byte, error)

func (r runnerFunc) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	return r(name + " " + strings.Join(args, " "))
}

func mustTaskXML(t *testing.T, s Spec) string {
	t.Helper()
	doc, err := TaskXML(s)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestInstall_TaskWhenADaemonAlreadyRuns(t *testing.T) {
	f := newFake("windows")
	if _, err := Install(t.Context(), f.host(), spec, false); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "/Run") {
			t.Errorf("must not start a second daemon: %s", c)
		}
	}
	if _, ok := f.files[xmlScratch]; ok {
		t.Error("the temporary XML file must be removed")
	}
}

func TestInstall_TaskCreateFailureCleansUp(t *testing.T) {
	f := newFake("windows")
	f.failOn = []string{"/Create"}
	if _, err := Install(t.Context(), f.host(), spec, true); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := f.files[xmlScratch]; ok {
		t.Error("the temporary XML file must be removed")
	}
}

func TestUninstall_Task(t *testing.T) {
	f := newFake("windows")
	f.out["schtasks.exe /Query /TN truss-daemon /XML"] = "<Task><Description>" + Marker + "</Description></Task>"
	if _, err := Uninstall(t.Context(), f.host()); err != nil {
		t.Fatal(err)
	}
	f.wantCalls(t,
		"schtasks.exe /Query /TN truss-daemon /XML",
		"schtasks.exe /End /TN truss-daemon",
		"schtasks.exe /Delete /TN truss-daemon /F",
	)
}

func TestUninstall_TaskNotCreatedByTrussIsLeftAlone(t *testing.T) {
	f := newFake("windows")
	f.out["schtasks.exe /Query /TN truss-daemon /XML"] = "<Task><Description>mine</Description></Task>"
	res, err := Uninstall(t.Context(), f.host())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(c, "/Delete") || strings.Contains(c, "/End") {
			t.Errorf("must not touch a foreign task: %s", c)
		}
	}
	if !strings.Contains(strings.Join(res.Messages, ""), "not installed") {
		t.Errorf("messages = %v", res.Messages)
	}
}

func TestInstalled(t *testing.T) {
	for goos, path := range map[string]string{"linux": unitFile, "darwin": plistFile} {
		f := newFake(goos)
		if ok, err := Installed(t.Context(), f.host()); ok || err != nil {
			t.Errorf("%s empty: %v %v", goos, ok, err)
		}
		f.files[path] = "# " + Marker
		if ok, err := Installed(t.Context(), f.host()); !ok || err != nil {
			t.Errorf("%s installed: %v %v", goos, ok, err)
		}
		if len(f.calls) != 0 {
			t.Errorf("%s: status must not run OS tools: %v", goos, f.calls)
		}
	}
	f := newFake("windows")
	f.out["schtasks.exe /Query /TN truss-daemon /XML"] = Marker
	if ok, _ := Installed(t.Context(), f.host()); !ok {
		t.Error("windows: want installed")
	}
	if _, err := Installed(t.Context(), newFake("plan9").host()); !errors.Is(err, ErrUnsupported) {
		t.Errorf("plan9: err = %v", err)
	}
}

func TestUnsupportedOS(t *testing.T) {
	h := newFake("freebsd").host()
	if h.Supported() {
		t.Error("freebsd must not be supported")
	}
	if _, err := Install(t.Context(), h, spec, true); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Install err = %v", err)
	}
	if _, err := Uninstall(t.Context(), h); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Uninstall err = %v", err)
	}
}
