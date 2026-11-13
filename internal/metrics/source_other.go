//go:build !linux && !windows

package metrics

import "runtime"

func newSource() (source, string) {
	return nil, "resource figures are not collected on " + runtime.GOOS + " yet"
}
