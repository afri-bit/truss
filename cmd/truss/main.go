// Command truss is the TRUSS command-line interface and, via its daemon
// subcommand, the trussd daemon.
//
// This bootstrap build only reports version information. The command
// surface is a public contract and is defined by its own specification.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
)

// Build metadata, injected at link time:
//
//	go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc1234 -X main.date=2026-01-01T00:00:00Z"
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// buildInfo is the version metadata reported by the binary.
type buildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

// currentBuildInfo returns the link-time metadata, falling back to the
// module and VCS data recorded by the Go toolchain when ldflags are absent
// (for example after `go install`).
func currentBuildInfo() buildInfo {
	bi := buildInfo{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bi
	}
	if bi.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		bi.Version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && bi.Commit == "none":
			bi.Commit = s.Value
		case s.Key == "vcs.time" && bi.Date == "unknown":
			bi.Date = s.Value
		}
	}
	return bi
}

// writeVersion writes bi in a stable, line-oriented format.
func writeVersion(w io.Writer, bi buildInfo) error {
	_, err := fmt.Fprintf(w, "truss %s\ncommit: %s\nbuilt: %s\ngo: %s\nplatform: %s\n",
		bi.Version, bi.Commit, bi.Date, bi.GoVersion, bi.Platform)
	if err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	return nil
}

func main() {
	if err := writeVersion(os.Stdout, currentBuildInfo()); err != nil {
		fmt.Fprintln(os.Stderr, "truss:", err)
		os.Exit(1)
	}
}
