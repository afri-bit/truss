package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// helperFlag is the first argument that turns the test binary into a helper
// process. Steps run it through the host shell, so no environment is needed.
const helperFlag = "-truss-helper"

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == helperFlag {
		os.Exit(runHelper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// helperCmd returns a run string for /bin/sh -c and cmd /c that runs the
// helper subcommand with args. Values with spaces are double-quoted.
func helperCmd(args ...string) string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	parts := []string{quoteArg(exe), helperFlag}
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if runtime.GOOS == "windows" && !strings.ContainsAny(s, " \t") {
		return s
	}
	return `"` + s + `"`
}

func runHelper(args []string) int {
	if len(args) == 0 {
		return 2
	}
	rest := args[1:]
	switch args[0] {
	case "touch-and-wait": // <mine> <other>
		if err := os.WriteFile(rest[0], nil, 0o600); err != nil {
			return 2
		}
		if pollFile(rest[1], 30*time.Second) {
			return 0
		}
		return 3
	case "print-lines": // <n> <label>
		n, _ := strconv.Atoi(rest[0])
		for i := 1; i <= n; i++ {
			fmt.Printf("%s %d\n", rest[1], i)
		}
		return 0
	case "partial": // <text>
		fmt.Print(rest[0])
		return 0
	case "exit": // <code>
		code, _ := strconv.Atoi(rest[0])
		return code
	case "spawn-grandchild": // <pidfile> [leave]
		return spawnGrandchild(rest)
	case "block": // <pidfile>
		writePid(rest[0])
		for {
			time.Sleep(time.Hour)
		}
	}
	return 2
}

func spawnGrandchild(rest []string) int {
	writePid(rest[0] + ".parent")
	exe, err := os.Executable()
	if err != nil {
		return 2
	}
	child := exec.Command(exe, helperFlag, "block", rest[0]) //nolint:gosec // test helper re-executes itself
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		return 2
	}
	if len(rest) > 1 && rest[1] == "leave" {
		if !pollFile(rest[0], 30*time.Second) {
			return 3
		}
		return 0
	}
	for {
		time.Sleep(time.Hour)
	}
}

// writePid writes the pid atomically so readers never see a partial file.
func writePid(path string) {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	_ = os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())), 0o600)
	_ = os.Rename(tmp, path)
}

func pollFile(path string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
