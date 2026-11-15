//go:build !linux && !windows && !darwin

package metrics

import "runtime"

func newSource() (source, string) {
	return nil, "resource figures are not collected on " + runtime.GOOS + " yet"
}
