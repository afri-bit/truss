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
	if strings.Contains(s.Binary, "%") {
		return "", errors.New("Task Scheduler expands %NAME% in a command, so a binary path containing % cannot be used; move truss to another directory")
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
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, r := range s {
		switch r {
		case '\\':
			slashes++
			b.WriteRune(r)
		case '"':
			b.WriteString(strings.Repeat(`\`, slashes+1)) // double the backslashes, escape the quote
			b.WriteRune(r)
			slashes = 0
		default:
			slashes = 0
			b.WriteRune(r)
		}
	}
	b.WriteString(strings.Repeat(`\`, slashes)) // backslashes before the closing quote
	b.WriteByte('"')
	return b.String()
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
func SchtasksCreateArgs(task, xmlPath string) []string {
	return []string{"/Create", "/TN", task, "/XML", xmlPath, "/F"}
}

// SchtasksQueryArgs prints the task definition as XML; it fails when there is no task.
func SchtasksQueryArgs(task string) []string { return []string{"/Query", "/TN", task, "/XML"} }

// SchtasksRunArgs starts the task now.
func SchtasksRunArgs(task string) []string { return []string{"/Run", "/TN", task} }

// SchtasksEndArgs stops the task's running instance.
func SchtasksEndArgs(task string) []string { return []string{"/End", "/TN", task} }

// SchtasksDeleteArgs deletes the task.
func SchtasksDeleteArgs(task string) []string { return []string{"/Delete", "/TN", task, "/F"} }

func (h Host) installTask(ctx context.Context, s Spec, startNow bool) (Result, error) {
	var res Result
	task := TaskName(h.User)
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
	if err := h.run(ctx, "schtasks.exe", SchtasksCreateArgs(task, xmlPath)...); err != nil {
		return res, fmt.Errorf("%w (some Windows setups only let administrators create a task that runs at logon; see docs/daemon.md)", err)
	}
	verb := "installed"
	if existed {
		verb = "updated"
	}
	res.add("login start %s: scheduled task %q (runs when you log on)", verb, task)
	if startNow {
		if err := h.run(ctx, "schtasks.exe", SchtasksRunArgs(task)...); err != nil {
			return res, fmt.Errorf("the task was installed but could not be started: %w", err)
		}
		res.Restarted = true
		res.add("the daemon was started through the task")
	} else {
		res.add("the daemon starts at your next logon; start it now with: schtasks /Run /TN %s", task)
		if existed {
			res.add("a running daemon keeps its old settings; to apply the change now: truss daemon stop, then truss daemon install again")
		}
	}
	if s.Home != "" {
		res.add("note: TRUSS_HOME is set in this shell but a task cannot carry it; the daemon uses the user variable of that name (setx) or the default home")
	}
	return res, nil
}

func (h Host) uninstallTask(ctx context.Context) (Result, error) {
	var res Result
	task := TaskName(h.User)
	ok, err := Installed(ctx, h)
	if err != nil {
		return res, err
	}
	if !ok {
		res.add("login start is not installed")
		return res, nil
	}
	_ = h.run(ctx, "schtasks.exe", SchtasksEndArgs(task)...) // not running is fine
	if err := h.run(ctx, "schtasks.exe", SchtasksDeleteArgs(task)...); err != nil {
		return res, err
	}
	res.add("login start removed: scheduled task %q", task)
	return res, nil
}
