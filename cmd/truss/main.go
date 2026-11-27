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
	// The first Ctrl-C (or SIGTERM) asks the steps to stop; a second one
	// kills them at once (spec 028).
	ctx, cancel := context.WithCancel(context.Background())
	killNow := make(chan struct{})
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		if isRun(args) {
			fmt.Fprintln(os.Stderr, "stopping the steps; press Ctrl-C again to kill them now")
		}
		cancel()
		<-signals
		close(killNow)
	}()
	code := RunWithInput(withKillNow(ctx, killNow), args, cwd, os.Environ(), os.Stdin, os.Stdout, os.Stderr)
	signal.Stop(signals)
	cancel()
	os.Exit(code)
}
