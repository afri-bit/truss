package main

import (
	"fmt"
	"os"

	"github.com/afri-bit/truss/internal/home"
	"github.com/afri-bit/truss/internal/registry"
)

// cache handles "truss cache clear <name>": it empties the cache folder the
// daemon keeps for a pipeline. It works on files only, so it does not need
// the daemon, and it works for a pipeline that is no longer registered.
func (in invocation) cache() int {
	if len(in.args) != 2 || in.args[0] != "clear" {
		return in.usage()
	}
	name := in.args[1]
	if err := registry.CheckName(name); err != nil {
		return in.fail(err)
	}
	h, err := home.Resolve(in.env)
	if err != nil {
		return in.fail(err)
	}
	dir := h.PipelineCache(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(in.stdout, "no cache for %s\n", name)
		return 0
	}
	if err := os.RemoveAll(dir); err != nil {
		return in.fail(fmt.Errorf("clear the cache of %s: %w", name, err))
	}
	fmt.Fprintf(in.stdout, "cleared the cache of %s\n", name)
	return 0
}
