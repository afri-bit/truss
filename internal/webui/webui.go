// Package webui holds TRUSS Desktop: a small web application the daemon
// serves from memory. It is plain HTML, CSS and JavaScript with no build
// step, and it uses the same HTTP API as the CLI.
package webui

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed assets
var assets embed.FS

//go:embed demo
var demo embed.FS

//go:embed VERSION
var version string

// Version is TRUSS Desktop's own version, kept in the VERSION file next to this package so that
// it is separate from the Go module version in go.mod. A release tag must match both (see
// .github/scripts/release-check.sh).
func Version() string { return strings.TrimSpace(version) }

// Files returns the application's files, with index.html at the root.
func Files() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // the directory is compiled in; this cannot fail at run time
	}
	return sub
}

// DemoFiles returns demo.js, the sample data for `truss desktop --demo`. The
// daemon does not serve it, so its pages can never show sample data.
func DemoFiles() fs.FS {
	sub, err := fs.Sub(demo, "demo")
	if err != nil {
		panic(err) // the directory is compiled in; this cannot fail at run time
	}
	return sub
}
