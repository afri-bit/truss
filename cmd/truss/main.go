package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(Run(context.Background(), os.Args[1:], cwd, os.Environ(), os.Stdout, os.Stderr))
}
