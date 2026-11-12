package loginstart

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (h Host) unitPath() string {
	config := h.ConfigHome
	if config == "" {
		config = filepath.Join(h.UserHome, ".config")
	}
	return filepath.Join(config, "systemd", "user", UnitName)
}

// SystemdUnit returns the text of the systemd user unit for s.
func SystemdUnit(s Spec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s. Remove it with \"truss daemon uninstall\".\n", Marker)
	b.WriteString("[Unit]\nDescription=TRUSS daemon\n\n[Service]\nType=simple\n")
	words := []string{systemdQuote(s.Binary, true)}
	for _, a := range s.Args() {
		words = append(words, systemdQuote(a, true))
	}
	fmt.Fprintf(&b, "ExecStart=%s\n", strings.Join(words, " "))
	if s.Home != "" {
		fmt.Fprintf(&b, "Environment=%s\n", systemdQuote("TRUSS_HOME="+s.Home, false))
	}
	b.WriteString("Restart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

// systemdQuote quotes one word of a unit file. Specifiers (%) are escaped
// everywhere; variable expansion ($) only in ExecStart.
func systemdQuote(s string, exec bool) string {
	safe := s != ""
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_@+=:,./-", r)) {
			safe = false
		}
	}
	if safe {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%")
	q := r.Replace(s)
	if exec {
		q = strings.ReplaceAll(q, "$", "$$")
	}
	return `"` + q + `"`
}

func systemctl(args ...string) []string { return append([]string{"--user"}, args...) }

func (h Host) installSystemd(ctx context.Context, s Spec, startNow bool) (Result, error) {
	var res Result
	path := h.unitPath()
	existed, err := h.hasMarker(path)
	if err != nil {
		return res, err
	}
	if err := h.Files.Write(path, []byte(SystemdUnit(s)), 0o644); err != nil {
		return res, fmt.Errorf("write %s: %w", path, err)
	}
	fail := func(err error) (Result, error) {
		if !existed {
			_ = h.run(ctx, "systemctl", systemctl("disable", UnitName)...)
			_ = h.Files.Remove(path)
			_ = h.run(ctx, "systemctl", systemctl("daemon-reload")...)
		}
		return res, fmt.Errorf("%w (is a systemd user session available?)", err)
	}
	if err := h.run(ctx, "systemctl", systemctl("daemon-reload")...); err != nil {
		return fail(err)
	}
	if err := h.run(ctx, "systemctl", systemctl("enable", UnitName)...); err != nil {
		return fail(err)
	}
	active := h.run(ctx, "systemctl", systemctl("is-active", "--quiet", UnitName)...) == nil
	if startNow || active {
		if err := h.run(ctx, "systemctl", systemctl("restart", UnitName)...); err != nil {
			return fail(err)
		}
		res.Restarted = true
	}
	verb := "installed"
	if existed {
		verb = "updated"
	}
	res.add("login start %s: %s", verb, path)
	switch {
	case startNow || active:
		res.add("the daemon is running under systemd and starts at every login")
	default:
		res.add("the daemon starts at your next login; start it now with: systemctl --user start %s", UnitName)
	}
	res.add("to run it without a login session, once: loginctl enable-linger")
	return res, nil
}

func (h Host) uninstallSystemd(ctx context.Context) (Result, error) {
	var res Result
	path := h.unitPath()
	ok, err := h.hasMarker(path)
	if err != nil {
		return res, err
	}
	if !ok {
		res.add("login start is not installed")
		return res, nil
	}
	if err := h.run(ctx, "systemctl", systemctl("disable", "--now", UnitName)...); err != nil {
		res.add("warning: %v", err)
	}
	if err := h.Files.Remove(path); err != nil {
		return res, fmt.Errorf("remove %s: %w", path, err)
	}
	_ = h.run(ctx, "systemctl", systemctl("daemon-reload")...)
	res.add("login start removed: %s", path)
	return res, nil
}
