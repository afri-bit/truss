// Package webui holds TRUSS Desktop: a small web application the daemon
// serves from memory. It is plain HTML, CSS and JavaScript with no build
// step, and it uses the same HTTP API as the CLI.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed assets
var assets embed.FS

// Files returns the application's files, with index.html at the root.
func Files() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // the directory is compiled in; this cannot fail at run time
	}
	return sub
}
