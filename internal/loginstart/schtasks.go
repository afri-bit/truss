package loginstart

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const taskFile = "login-task.xml"

// TaskXML returns the Task Scheduler definition for s. Importing XML rather than
// passing flags to schtasks is deliberate: it is the only way to turn off the
// 72-hour run limit and the stop-on-battery default that schtasks /Create sets.
func TaskXML(s Spec) (string, error) {
	if s.User == "" {
		return "", errors.New("no user name for the scheduled task")
	}
	if s.Home != "" {
		return "", errors.New("a custom TRUSS_HOME cannot be passed to a Windows login task; unset TRUSS_HOME or set it for your user with setx")
	}
	user := xmlText(s.User)
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-16\"?>\n")
	b.WriteString("<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n")
	fmt.Fprintf(&b, "  <RegistrationInfo>\n    <Description>%s. Remove it with \"truss daemon uninstall\".</Description>\n  </RegistrationInfo>\n", xmlText(Marker))
	fmt.Fprintf(&b, "  <Triggers>\n    <LogonTrigger>\n      <Enabled>true</Enabled>\n      <UserId>%s</UserId>\n    </LogonTrigger>\n  </Triggers>\n", user)
	fmt.Fprintf(&b, "  <Principals>\n    <Principal id=\"Author\">\n      <UserId>%s</UserId>\n      <LogonType>InteractiveToken</LogonType>\n      <RunLevel>LeastPrivilege</RunLevel>\n    </Principal>\n  </Principals>\n", user)
	b.WriteString("  <Settings>\n")
	b.WriteString("    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n")
	b.WriteString("    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\n")
	b.WriteString("    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\n")
	b.WriteString("    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>\n")
	b.WriteString("    <RestartOnFailure>\n      <Interval>PT1M</Interval>\n      <Count>3</Count>\n    </RestartOnFailure>\n")
	b.WriteString("    <Enabled>true</Enabled>\n  </Settings>\n")
	args := make([]string, 0, 6)
	for _, a := range s.Args() {
		args = append(args, windowsArg(a))
	}
	fmt.Fprintf(&b, "  <Actions Context=\"Author\">\n    <Exec>\n      <Command>%s</Command>\n      <Arguments>%s</Arguments>\n    </Exec>\n  </Actions>\n", xmlText(s.Binary), xmlText(strings.Join(args, " ")))
	b.WriteString("</Task>\n")
	return b.String(), nil
}

// windowsArg quotes one argument of a Windows command line.
func windowsArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// encodeUTF16 returns s as UTF-16 little endian with a byte order mark, the
// encoding schtasks /XML reads reliably.
func encodeUTF16(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2+2*len(units))
	out = append(out, 0xFF, 0xFE)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// The schtasks.exe argument lists install and uninstall use.

// SchtasksCreateArgs imports the task definition in xmlPath, replacing an older one.
func SchtasksCreateArgs(xmlPath string) []string {
	return []string{"/Create", "/TN", TaskName, "/XML", xmlPath, "/F"}
}

// SchtasksQueryArgs prints the task definition as XML; it fails when there is no task.
func SchtasksQueryArgs() []string { return []string{"/Query", "/TN", TaskName, "/XML"} }

// SchtasksRunArgs starts the task now.
func SchtasksRunArgs() []string { return []string{"/Run", "/TN", TaskName} }

// SchtasksEndArgs stops the task's running instance.
func SchtasksEndArgs() []string { return []string{"/End", "/TN", TaskName} }

// SchtasksDeleteArgs deletes the task.
func SchtasksDeleteArgs() []string { return []string{"/Delete", "/TN", TaskName, "/F"} }

func (h Host) installTask(ctx context.Context, s Spec, startNow bool) (Result, error) {
	var res Result
	doc, err := TaskXML(s)
	if err != nil {
		return res, err
	}
	existed, err := Installed(ctx, h)
	if err != nil {
		return res, err
	}
	xmlPath := filepath.Join(h.Scratch, taskFile)
	if err := h.Files.Write(xmlPath, encodeUTF16(doc), 0o600); err != nil {
		return res, fmt.Errorf("write %s: %w", xmlPath, err)
	}
	defer func() { _ = h.Files.Remove(xmlPath) }()
	if err := h.run(ctx, "schtasks.exe", SchtasksCreateArgs(xmlPath)...); err != nil {
		return res, err
	}
	verb := "installed"
	if existed {
		verb = "updated"
	}
	res.add("login start %s: scheduled task %q (runs when you log on)", verb, TaskName)
	if startNow {
		if err := h.run(ctx, "schtasks.exe", SchtasksRunArgs()...); err != nil {
			return res, err
		}
		res.add("the daemon was started through the task")
	} else {
		res.add("the daemon starts at your next logon; start it now with: schtasks /Run /TN %s", TaskName)
	}
	return res, nil
}

func (h Host) uninstallTask(ctx context.Context) (Result, error) {
	var res Result
	ok, err := Installed(ctx, h)
	if err != nil {
		return res, err
	}
	if !ok {
		res.add("login start is not installed")
		return res, nil
	}
	_ = h.run(ctx, "schtasks.exe", SchtasksEndArgs()...) // not running is fine
	if err := h.run(ctx, "schtasks.exe", SchtasksDeleteArgs()...); err != nil {
		return res, err
	}
	res.add("login start removed: scheduled task %q", TaskName)
	return res, nil
}
