// Command truss is the TRUSS command line: it runs pipelines, manages the daemon and opens TRUSS Desktop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	args := os.Args[1:]
	cwd, err := os.Getwd()
	if err != nil && isRun(args) {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := RunWithInput(ctx, args, cwd, os.Environ(), os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
