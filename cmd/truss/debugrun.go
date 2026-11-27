package main

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/afri-bit/truss/internal/pipeline"
)

// The options of `truss run` that debug a flow (spec 023).

// runOptions are the options of `truss run`.
type runOptions struct {
	track     bool
	notify    bool
	withNeeds bool
	step      string
	from      string
	// env holds the --env overrides, name to value.
	env map[string]string
}

// debug reports whether the run is a slice of the flow.
func (o runOptions) debug() bool { return o.step != "" || o.from != "" }

// selection returns the part of the flow to run and the overrides.
func (o runOptions) selection() pipeline.Selection {
	sel := pipeline.Selection{Env: o.env}
	switch {
	case o.step != "" && o.withNeeds:
		sel.Step, sel.Mode = o.step, pipeline.WithNeeds
	case o.step != "":
		sel.Step = o.step
	case o.from != "":
		sel.Step, sel.Mode = o.from, pipeline.From
	}
	return sel
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseRunArgs splits the arguments of `run` into the flow and its options.
// --track, --with-needs, --step, --from and --env are options; any other
// argument is the path, so a file named like a flag still runs (spec 001).
// An --env that is malformed never has its text echoed: it may hold a secret
// pasted by mistake.
func parseRunArgs(args []string) ([]string, runOptions, error) {
	var pos []string
	o := runOptions{env: map[string]string{}}
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		switch name {
		case "--track":
			if inline {
				pos = append(pos, args[i])
				continue
			}
			o.track = true
		case "--with-needs":
			if inline {
				pos = append(pos, args[i])
				continue
			}
			o.withNeeds = true
		case "--notify":
			if inline {
				pos = append(pos, args[i])
				continue
			}
			o.notify = true
		case "--step", "--from", "--env":
			if !inline {
				if i+1 >= len(args) {
					return nil, o, fmt.Errorf("%s needs a value", name)
				}
				i++
				value = args[i]
			}
			switch name {
			case "--step", "--from":
				target := &o.step
				if name == "--from" {
					target = &o.from
				}
				if *target != "" {
					return nil, o, fmt.Errorf("%s is given twice", name)
				}
				if value == "" {
					return nil, o, fmt.Errorf("%s needs a step id", name)
				}
				*target = value
			case "--env":
				k, v, ok := strings.Cut(value, "=")
				if !ok || !envName.MatchString(k) {
					return nil, o, errors.New("--env needs NAME=VALUE, with a name of letters, digits and _ that does not start with a digit")
				}
				pipeline.SetEnv(o.env, k, v)
			}
		default:
			pos = append(pos, args[i])
		}
	}
	switch {
	case o.step != "" && o.from != "":
		return nil, o, errors.New("--step and --from cannot be used together")
	case o.withNeeds && o.step == "":
		return nil, o, errors.New("--with-needs goes with --step")
	case o.debug() && o.track:
		return nil, o, errors.New("--track cannot be combined with --step or --from: a debug run is not reported to the daemon")
	}
	return pos, o, nil
}

// plainWord reports whether s is safe to print in a command line to paste:
// letters, digits and _ . / : @ + = - (and \ on Windows, where it separates
// paths). It does not start with - (an option), @ (PowerShell splatting) or =
// (zsh expands it to a command's path). A comma is out too: PowerShell reads
// it as an array. Quoting differs between sh, cmd and PowerShell, and one of
// them would still expand a $ or %, so anything else is not printed as a
// command.
func plainWord(s string) bool {
	if s == "" || strings.ContainsRune("-@=", rune(s[0])) {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("_./:@+=-", r):
		case r == '\\' && runtime.GOOS == "windows":
		default:
			return false
		}
	}
	return true
}

// rerunHint is the line printed after a failed run: the command that runs
// only the failed step. When the flow or the id holds anything but a plain
// word it names the option and leaves the command out.
func rerunHint(flow, step string) string {
	if plainWord(flow) && plainWord(step) {
		return "to run only this step again: truss run " + flow + " --step " + step
	}
	return "to run only the failed step again, use truss run --step with its id (quote it for your shell)"
}
