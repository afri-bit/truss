package main

import (
	"fmt"

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
	existed, err := h.ClearPipelineCache(name)
	if err != nil {
		return in.fail(err)
	}
	if !existed {
		fmt.Fprintf(in.stdout, "no cache for %s\n", name)
		return 0
	}
	fmt.Fprintf(in.stdout, "cleared the cache of %s\n", name)
	return 0
}
