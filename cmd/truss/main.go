package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	cwd, err := os.Getwd()
	if err != nil && isRun(args) {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(Run(context.Background(), args, cwd, os.Environ(), os.Stdout, os.Stderr))
}
