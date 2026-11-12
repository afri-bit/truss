//go:build !linux && !darwin && !windows

package main

import (
	"errors"
	"os"
)

// isTerminal is false here: without a hidden prompt the value is read from
// standard input as written.
func isTerminal(*os.File) bool { return false }

func disableEcho(*os.File) (func(), error) {
	return nil, errors.New("hidden input is not supported on this system")
}
