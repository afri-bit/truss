package loginstart

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (h Host) plistPath() string {
	return filepath.Join(h.UserHome, "Library", "LaunchAgents", Label+".plist")
}

func (h Host) domain() string { return "gui/" + strconv.Itoa(h.UID) }

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// LaunchdPlist returns the text of the launchd agent for s. KeepAlive restarts
// the daemon only after a failure, so "truss daemon stop" is not undone.
func LaunchdPlist(s Spec) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	fmt.Fprintf(&b, "<!-- %s. Remove it with \"truss daemon uninstall\". -->\n", xmlText(Marker))
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	fmt.Fprintf(&b, "\t<key>Label</key>\n\t<string>%s</string>\n", Label)
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlText(s.Binary))
	for _, a := range s.Args() {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlText(a))
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	b.WriteString("\t<key>ThrottleInterval</key>\n\t<integer>30</integer>\n")
	if s.LogPath != "" {
		fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", xmlText(s.LogPath))
		fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", xmlText(s.LogPath))
	}
	if s.Home != "" {
		fmt.Fprintf(&b, "\t<key>EnvironmentVariables</key>\n\t<dict>\n\t\t<key>TRUSS_HOME</key>\n\t\t<string>%s</string>\n\t</dict>\n", xmlText(s.Home))
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func (h Host) installLaunchd(ctx context.Context, s Spec, startNow bool) (Result, error) {
	var res Result
	path := h.plistPath()
	existed, err := h.hasMarker(path)
	if err != nil {
		return res, err
	}
	if err := h.Files.Write(path, []byte(LaunchdPlist(s)), 0o644); err != nil {
		return res, fmt.Errorf("write %s: %w", path, err)
	}
	target := h.domain() + "/" + Label
	loaded := h.run(ctx, "launchctl", "print", target) == nil
	if loaded {
		_ = h.run(ctx, "launchctl", "bootout", target)
	}
	if startNow || loaded {
		if err := h.bootstrap(ctx, path, loaded); err != nil {
			if !existed {
				_ = h.Files.Remove(path)
			}
			return res, fmt.Errorf("%w (is a graphical login session available?)", err)
		}
	}
	verb := "installed"
	if existed {
		verb = "updated"
	}
	res.add("login start %s: %s", verb, path)
	if startNow || loaded {
		res.add("the daemon is running under launchd and starts at every login")
	} else {
		res.add("the daemon starts at your next login; start it now with: launchctl bootstrap %s %s", h.domain(), path)
	}
	return res, nil
}

func (h Host) uninstallLaunchd(ctx context.Context) (Result, error) {
	var res Result
	path := h.plistPath()
	ok, err := h.hasMarker(path)
	if err != nil {
		return res, err
	}
	if !ok {
		res.add("login start is not installed")
		return res, nil
	}
	_ = h.run(ctx, "launchctl", "bootout", h.domain()+"/"+Label) // not loaded is fine
	if err := h.Files.Remove(path); err != nil {
		return res, fmt.Errorf("remove %s: %w", path, err)
	}
	res.add("login start removed: %s", path)
	return res, nil
}

// bootstrap loads the agent. Right after a bootout, launchd may still be
// unloading the old one and bootstrap fails with an I/O error, so a reload
// retries a few times.
func (h Host) bootstrap(ctx context.Context, path string, reload bool) error {
	attempts := 1
	if reload {
		attempts = 5
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = h.run(ctx, "launchctl", "bootstrap", h.domain(), path); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(h.Wait):
		}
	}
	return err
}
